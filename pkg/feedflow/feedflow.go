package feedflow

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aigic8/feedflow/internal/db"
	"github.com/aigic8/feedflow/internal/utils"
	"github.com/go-co-op/gocron/v2"
	"github.com/mmcdole/gofeed"
	"gorm.io/driver/postgres"
)

const DEFAULT_NOTIFY_TIMEOUT = 10 * time.Second

type FeedFlow struct {
	CronSchedule  string
	FeedsPath     string
	ConfigPath    string
	NotifyTimeout time.Duration
	db            *db.DB
	notify        *utils.DiscordNotify
}

func (f *FeedFlow) Run() {
	if err := f.setup(); err != nil {
		log.Fatalf("ERROR: setup: %v", err)
	}

	cronSession, err := gocron.NewScheduler()
	if err != nil {
		log.Fatalf("ERROR: creating cron session: %v\n", err)
	}

	feedURLs, err := f.readFeeds()
	if err != nil {
		log.Fatalf("ERROR: reading feeds: %v\n", err)
	}
	if err = f.updateFeeds(feedURLs); err != nil {
		log.Fatalf("ERROR: updating feeds: %v\n", err)
	}

	jobDef := gocron.CronJob(f.CronSchedule, false)
	task := gocron.NewTask(f.checkForArticles)
	if _, err = cronSession.NewJob(jobDef, task); err != nil {
		log.Fatalf("creating a new job: %v", err)
	}
	cronSession.Start()
	wg := new(sync.WaitGroup)
	wg.Add(1)
	wg.Wait()
}

func (f *FeedFlow) setup() error {
	if f.NotifyTimeout == 0 {
		f.NotifyTimeout = DEFAULT_NOTIFY_TIMEOUT
	}
	config, err := utils.ReadAndValidateConfig(f.ConfigPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	f.notify = utils.NewDiscordNotify(config.BotToken, []string{config.ChannelID}, f.NotifyTimeout)

	appDB, err := db.NewDB(postgres.Open(config.DbURI))
	if err != nil {
		return fmt.Errorf("creating db: %w", err)
	}
	if err = appDB.AutoMigrate(); err != nil {
		return fmt.Errorf("auto-migrating db: %v", err)
	}
	f.db = appDB

	return nil
}

func (f *FeedFlow) checkForArticles() {
	feedURLs, err := f.readFeeds()
	if err != nil {
		log.Printf("ERROR: reading feeds: %v\n", err)
		return
	}
	if err = f.updateFeeds(feedURLs); err != nil {
		log.Printf("ERROR: updating feeds: %v\n", err)
		return
	}

	for _, feedURL := range feedURLs {
		fp := gofeed.NewParser()
		feed, err := fp.ParseURL(feedURL)
		if err != nil {
			log.Printf("ERROR: reading feed '%s': %v\n", feedURL, err)
			continue
		}

		dbFeed, err := f.db.FeedGet(feedURL)
		if err != nil {
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
				if err := f.notify.Send(messageContent); err != nil {
					log.Printf("sending notification: %v\n", err)
					continue
				}
			}
		}

		if err = f.db.FeedSetLastChecked(dbFeed.URL, time.Now()); err != nil {
			log.Printf("error setting last checked for feed '%s': %v\n", feedURL, err)
			continue
		}
	}
}

func (f *FeedFlow) updateFeeds(feedURLs []string) error {
	feedUpdateResult, err := f.db.FeedUpdateMany(feedURLs)
	if err != nil {
		return fmt.Errorf("updating feeds: %w", err)
	}

	addedFeedsLen := len(feedUpdateResult.AddedFeeds)
	if addedFeedsLen > 0 {
		addedFeedURLs := getFeedURLs(feedUpdateResult.AddedFeeds)
		addedFeedURLsStr := strings.Join(addedFeedURLs, "\n")

		if err := f.notify.Send(fmt.Sprintf("%d feed(s) were added:\n%s", addedFeedsLen, addedFeedURLsStr)); err != nil {
			return fmt.Errorf("notifying: %w", err)
		}
	}

	deactivatedFeedsLen := len(feedUpdateResult.DeactivatedFeeds)
	if deactivatedFeedsLen > 0 {
		deactivatedFeedURLs := getFeedURLs(feedUpdateResult.DeactivatedFeeds)
		deactivatedFeedURLsStr := strings.Join(deactivatedFeedURLs, "\n")

		if err := f.notify.Send(fmt.Sprintf("%d feed(s) were deactivated:\n%s", deactivatedFeedsLen, deactivatedFeedURLsStr)); err != nil {
			return fmt.Errorf("notifying: %w", err)
		}
	}

	return nil
}

func (f *FeedFlow) readFeeds() ([]string, error) {
	feedsFile, err := os.Open(f.FeedsPath)
	if err != nil {
		return nil, fmt.Errorf("opening feeds file: %w", err)
	}
	feedsFileBytes, err := io.ReadAll(feedsFile)
	if err != nil {
		return nil, fmt.Errorf("reading feeds file: %w", err)
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

	return feedURLs, nil
}

func getFeedURLs(feeds []db.Feed) []string {
	feedURLs := []string{}
	for _, feed := range feeds {
		feedURLs = append(feedURLs, feed.URL)
	}
	return feedURLs
}
