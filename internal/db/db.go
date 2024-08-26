package db

import (
	"fmt"
	"time"

	"github.com/guregu/null/v5"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DB struct {
	core *gorm.DB
}

type (
	Feed struct {
		gorm.Model
		URL           string `gorm:"unique"`
		LastChecked   time.Time
		DeactivatedAt null.Time
		LastSeen      time.Time `gorm:"not null"`
	}

	UpdateFeedsResult struct {
		DeactivatedFeeds []Feed
		AddedFeeds       []Feed
		// NOT IMPLEMENTED YET! DO NOT USE
		ActivatedFeeds []Feed
	}
)

func NewDB(driver gorm.Dialector) (*DB, error) {
	db, err := gorm.Open(driver, &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		return nil, err
	}
	return &DB{core: db}, nil
}

func (db *DB) AutoMigrate() error {
	return db.core.AutoMigrate(&Feed{})
}

func (db *DB) FeedGet(feedURL string) (*Feed, error) {
	var feed Feed
	if res := db.core.Where("url = ? AND deactivated_at is NULL", feedURL).First(&feed); res.Error != nil {
		return nil, fmt.Errorf("getting feed '%s': %w\n", feedURL, res.Error)
	}
	return &feed, nil
}

func (db *DB) FeedSetLastChecked(feedURL string, time time.Time) error {
	return db.core.Where("url = %s").Update("last_checked", time).Error
}

func (db *DB) FeedCreatedAfterMany(afterTime time.Time) ([]Feed, error) {
	var feeds []Feed
	if res := db.core.Where("created_at > ?", afterTime).Find(&feeds); res.Error != nil {
		return nil, res.Error
	}
	return feeds, nil
}

func (db *DB) FeedDeactivatedAfterMany(afterTime time.Time) ([]Feed, error) {
	var feeds []Feed
	if res := db.core.Where("deactivated_at > ?", afterTime).Find(&feeds); res.Error != nil {
		return nil, res.Error
	}
	return feeds, nil
}

func (db *DB) FeedUpdateMany(feedURLs []string) (*UpdateFeedsResult, error) {
	startTime := time.Now()
	err := db.core.Transaction(func(tx *gorm.DB) error {
		for _, feedURL := range feedURLs {
			res := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "url"}},
				DoUpdates: clause.Assignments(map[string]interface{}{
					"last_seen":      startTime,
					"deactivated_at": null.NewTime(startTime, false),
				})},
			).Create(&Feed{URL: feedURL, LastChecked: startTime, LastSeen: startTime})
			if res.Error != nil {
				// TODO: check if error is already exists, then return do not log the error
				// good source: https://github.com/go-gorm/gorm/issues/4037
				return fmt.Errorf("inserting feed '%s': %w", feedURL, res.Error)
			}
		}
		if err := tx.Where("last_seen < ? AND deactivated_at is NULL", startTime).Update("deactivated_at", startTime).Error; err != nil {
			return fmt.Errorf("ERROR: deactivating feeds: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("updating feeds: %w", err)
	}

	addedFeeds, err := db.FeedCreatedAfterMany(startTime)
	if err != nil {
		return nil, fmt.Errorf("getting added feeds: %w", err)
	}

	deactivatedFeeds, err := db.FeedDeactivatedAfterMany(startTime)
	if err != nil {
		return nil, fmt.Errorf("getting deactivated feeds: %w", err)
	}

	return &UpdateFeedsResult{
		AddedFeeds:       addedFeeds,
		DeactivatedFeeds: deactivatedFeeds,
		ActivatedFeeds:   []Feed{},
	}, nil
}
