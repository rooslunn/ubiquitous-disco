package rss_reader

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"

	"github.com/mmcdole/gofeed"
)

const (
	SERVICE_DIR = "./.sputnik"
)

const (
	E_GET_FEED_FILE = iota + 1
	E_READ_FEED_FILE
	E_UPDATE_FEED_FILE
	E_ENCDOING_UNPROCESSED
)

var (
	ErrFeedInvalidJson = errors.New("invalid json")
)

// [x] main log
// [ ] tests
// [ ] concurrency
// [ ] env config
// [ ] send tg message
// [ ] post middlewares (translate, expand, picturize, etc.)

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

	feeds, err := loadFeeds(userFeedsFile)
	if err != nil {
		log.Error(err.Error())
		os.Exit(E_READ_FEED_FILE)
	}

	feedParser := gofeed.NewParser()

	for _, userFeed := range feeds.Items {
		ctx := context.Background()	
		err = getUpdates(ctx, feedParser, userFeed, log)
		if err != nil {
			log.Error(err.Error())
		}
	}

	log.Info("saving updates to", "path", userFeedsFile)

	err = saveUpdates(feeds, userFeedsFile)
	if err != nil {
		log.Error(err.Error())
		os.Exit(E_UPDATE_FEED_FILE)
	}

	log.Info("updates saved to", "path", userFeedsFile)

	log.Info("session done")

}

func loadFeeds(userFeedsFile string) (Feeds, error) {
	file, err := os.Open(userFeedsFile)
	if err != nil {
		return Feeds{}, err
	}
	defer file.Close()

	var feeds Feeds
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&feeds); err != nil {
		return Feeds{}, err
	}

	if feeds.Items == nil {
		return Feeds{}, ErrFeedInvalidJson
	}

	return feeds, nil
}

func getUpdates(_ context.Context, feedParser FeedFetcher, userFeed *Feed, log *slog.Logger) error {

		log.Info("processing feed", "url", userFeed.Url, "updated", userFeed.Updated)

		remoteFeed, err := feedParser.ParseURL(userFeed.Url)
		if err != nil {
			return err
		}

		if remoteFeed.Updated != userFeed.Updated {
			log.Info("got updates", "count", len(remoteFeed.Items))
			userFeed.Updated = remoteFeed.Updated
			newFeeds := 0

			for _, remoteItem := range remoteFeed.Items {
				if _,  exists := userFeed.UnprocessedGUID[remoteItem.GUID]; !exists {
					log.Info("new post", "guid", remoteItem.GUID, "title", firstNRunes(remoteItem.Title, 64), "updated", remoteItem.Updated)
					userFeed.UnprocessedGUID[remoteItem.GUID] = struct{}{}
					userFeed.UnprocessedItems = append(userFeed.UnprocessedItems, &UnprocessedItem{
						GUID: remoteItem.GUID,
						URL:  remoteItem.Link,
					})
					newFeeds++
				}
			}
			log.Info("total new feeds", "count", newFeeds)
		} else {
			log.Info("no new items")
		}
	return nil
}

func saveUpdates(feeds Feeds, userFeedsFile string) error {
	file, err := os.Create(userFeedsFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(feeds); err != nil {
		return err
	}

	return nil
}
