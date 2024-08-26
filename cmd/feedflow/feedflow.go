package main

import (
	"log"
	"os"
	"path"
	"time"

	"github.com/aigic8/feedflow/pkg/feedflow"
)

const NOTIFY_TIMEOUT = 10 * time.Second

const FEEDS_FILE_PATH = "feedflow/feeds.txt"
const CONFIG_PATH = "feedflow/config.yaml"

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("getting working directory: %v", err)
	}

	cronSchedule := "0 */6 * * *"
	feedsPath := path.Join(wd, FEEDS_FILE_PATH)
	configPath := path.Join(wd, CONFIG_PATH)

	ff := feedflow.FeedFlow{
		CronSchedule:  cronSchedule,
		FeedsPath:     feedsPath,
		ConfigPath:    configPath,
		NotifyTimeout: NOTIFY_TIMEOUT,
	}
	ff.Run()
}
