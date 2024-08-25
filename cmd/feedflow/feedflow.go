package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/go-playground/validator/v10"
	"github.com/mmcdole/gofeed"
	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/discord"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const DISCORD_TIMEOUT = 10 * time.Second

const FEEDS_FILE_PATH = "feedflow/feeds.txt"
const CONFIG_PATH = "feedflow/config.yaml"

type (
	Feed struct {
		gorm.Model
		URL         string `gorm:"unique"`
		LastChecked time.Time
	}

	Config struct {
		BotToken  string `validate:"required" yaml:"botToken"`
		ChannelID string `validate:"required" yaml:"channelID"`
		DbURI     string `validate:"required" yaml:"dbURI"`
	}
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("getting working directory: %v", err)
	}
	config, err := readConfig(path.Join(wd, CONFIG_PATH))
	if err != nil {
		log.Fatalf("reading config: %v", err)
	}

	discordService := discord.New()
	discordService.AuthenticateWithBotToken(config.BotToken)
	discordService.AddReceivers(config.ChannelID)
	db, err := gorm.Open(postgres.Open(config.DbURI), &gorm.Config{})
	if err != nil {
		log.Fatalf("connecting to db: %v", err)
	}
	appNotify := notify.New()
	appNotify.UseServices(discordService)

	db.AutoMigrate(&Feed{})

	cronSession, err := gocron.NewScheduler()
	if err != nil {
		log.Fatalf("creating cron session: %v", err)
	}
	app := App{db: db, notify: appNotify}
	// TODO: refactor
	feedURLs := app.readFeeds()
	app.updateFeeds(feedURLs)
	jobDef := gocron.CronJob("0 */6 * * *", false)
	task := gocron.NewTask(app.checkForArticles)
	if _, err = cronSession.NewJob(jobDef, task); err != nil {
		log.Fatalf("creating a new job: %v", err)
	}
	cronSession.Start()
	wg := new(sync.WaitGroup)
	wg.Add(1)
	wg.Wait()
}

type App struct {
	db     *gorm.DB
	notify *notify.Notify
}

func (app App) checkForArticles() {
	feedURLs := app.readFeeds()
	app.updateFeeds(feedURLs)

	for _, feedURL := range feedURLs {
		fp := gofeed.NewParser()
		feed, err := fp.ParseURL(feedURL)
		if err != nil {
			log.Printf("ERROR: reading feed '%s': %v\n", feedURL, err)
			continue
		}

		var dbFeed Feed
		if res := app.db.Where(&Feed{URL: feedURL}).First(&dbFeed); res.Error != nil {
			log.Printf("ERROR: getting feed '%s' from db: %v\n", feedURL, err)
			continue
		}

		for _, item := range feed.Items {
			if item.PublishedParsed == nil {
				log.Printf("no published date in feed '%s'\n", feedURL)
				continue
			}
			if item.PublishedParsed.After(dbFeed.LastChecked) {
				messageContent := fmt.Sprintf("%s\n%s", item.Title, item.Link)
				ctx, cancel := context.WithTimeout(context.Background(), DISCORD_TIMEOUT)
				defer cancel()
				if err := app.notify.Send(ctx, "", messageContent); err != nil {
					log.Printf("sending notification: %v\n", err)
					continue
				}
			}
		}

		dbFeed.LastChecked = time.Now()
		if res := app.db.Save(&dbFeed); res.Error != nil {
			log.Printf("error updating feed '%s' in db: %v\n", feedURL, err)
			continue
		}
	}
}

func readConfig(configPath string) (*Config, error) {
	configFile, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}

	configBytes, err := io.ReadAll(configFile)
	if err != nil {
		return nil, err
	}

	var config Config
	if err = yaml.Unmarshal(configBytes, &config); err != nil {
		return nil, err
	}

	v := validator.New(validator.WithRequiredStructEnabled())
	if err = v.Struct(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// TODO: refactor should return error too
func (app *App) updateFeeds(feedURLs []string) {
	for _, feedURL := range feedURLs {
		// TODO: use batch insert instead
		res := app.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&Feed{URL: feedURL, LastChecked: time.Now()})
		if res.Error != nil {
			log.Printf("ERROR: inserting feed '%s': %v\n", feedURL, res.Error)
		}
		ctx, cancel := context.WithTimeout(context.Background(), DISCORD_TIMEOUT)
		defer cancel()
		if err := app.notify.Send(ctx, "", fmt.Sprintf("feed '%s' was added", feedURL)); err != nil {
			log.Printf("ERROR: notifying %v\n", err)
		}
	}
}

// TODO: refactor should return error too
func (app *App) readFeeds() []string {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("getting working directory: %v", err)
	}
	feedURLsFilePath := path.Join(wd, FEEDS_FILE_PATH)
	feedsFile, err := os.Open(feedURLsFilePath)
	if err != nil {
		log.Fatalf("opening feeds file: %v", err)
	}
	feedsFileBytes, err := io.ReadAll(feedsFile)
	if err != nil {
		log.Fatalf("reading feeds file: %v", err)
	}

	lines := strings.Split(string(feedsFileBytes), "\n")
	feedURLs := []string{}
	for _, line := range lines {
		feedURL := strings.TrimSpace(line)
		if feedURL == "" {
			continue
		}
		feedURLs = append(feedURLs, feedURL)
	}

	return feedURLs
}
