package main

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/mmcdole/gofeed"
)

type Feeds struct {
	Version string  `json:"version"`
	Items   []*Feed `json:"items"`
}

type UnprocessedItem struct {
	URL  string `json:"url"`
	GUID string `json:"guid"`
}

type Feed struct {
	Type             string             `json:"type"`
	Hash             string             `json:"hash"`
	Url              string             `json:"url"`
	Updated          string             `json:"updated"`
	UnprocessedSet   string             `json:"unprocessed_set"`
	UnprocessedItems []*UnprocessedItem `json:"unprocessed_items"`
}

const (
	SERVICE_DIR = "./.sputnik"
)

const (
	E_GET_FEED_FILE = iota + 1
	E_READ_FEED_FILE
	E_DECODE_FEED_FILE
	E_OPEN_FEED_FILE_UPDATE
	E_ENCDOING_UNPROCESSED
)

// todo: main log
// todo: env config
// todo: tests
// todo: send tg message
// todo: post mmiddlewares (translate, expand, picturize, etc.)

func main() {

	log := setupLogger()

	user_id := "rkladko@gmail.com"
	user_hash := GetSHA256(user_id)
	log.Info("user is ready", "id", user_id, "hash", user_hash)

	userFeedsFile, err := GetFeedsFile(user_hash)
	if err != nil {
		log.Error(err.Error())
		os.Exit(E_GET_FEED_FILE)
	}
	log.Info("user feed file", "path", userFeedsFile)


	file, err := os.Open(userFeedsFile)
	if err != nil {
		log.Error(err.Error())
		os.Exit(E_READ_FEED_FILE)
	}
	defer file.Close()

	var feeds Feeds
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&feeds); err != nil {
		log.Error(err.Error())
		os.Exit(E_DECODE_FEED_FILE)
	}

	for _, userFeed := range feeds.Items {

		log.Info("user feed", "hash", userFeed.Hash, "updated", userFeed.Updated, "url", userFeed.Url)

		feedParser := gofeed.NewParser()
		remoteFeed, err := feedParser.ParseURL(userFeed.Url)
		if err != nil {
			log.Error(err.Error())
			continue
		}

		if remoteFeed.Updated != userFeed.Updated {

			log.Info("download feed updates", "received", len(remoteFeed.Items))

			userFeed.Updated = remoteFeed.Updated
			newFeeds := 0

			for _, remoteItem := range remoteFeed.Items {
				if !strings.Contains(userFeed.UnprocessedSet, remoteItem.GUID) {
					log.Info("new post", "guid", remoteItem.GUID, "title", firstNRunes(remoteItem.Title, 64), "updated", remoteItem.Updated)
					userFeed.UnprocessedSet += "," + remoteItem.GUID
					userFeed.UnprocessedItems = append(userFeed.UnprocessedItems, &UnprocessedItem{
						GUID: remoteItem.GUID,
						URL:  remoteItem.Link,
					})
					newFeeds++
				}
			}
			log.Info("total new feeds receieved", "total", newFeeds)
		} else {
			log.Info("no new items")
		}

	}

	log.Info("saving unprocessed", "path", userFeedsFile)

	file, err = os.Create(userFeedsFile)
	if err != nil {
		log.Error(err.Error())
		os.Exit(E_OPEN_FEED_FILE_UPDATE)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(feeds); err != nil {
		log.Error(err.Error())
		os.Exit(E_ENCDOING_UNPROCESSED)
	}

	log.Info("session done")

}
