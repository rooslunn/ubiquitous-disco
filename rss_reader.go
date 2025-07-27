package rss_reader

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mmcdole/gofeed"
	"golang.org/x/sync/errgroup"
)

const (
	SERVICE_DIR = "./.sputnik"
	maxConcurrentFeeds = 5
)

const (
	E_GET_FEED_FILE = iota + 1
	E_READ_FEED_FILE
	E_UPDATE_FEED_FILE
	E_ENCDOING_UNPROCESSED
	E_CONCURRENT_FAILURE
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
	realFeedsIO := &RealFeedsIO{}
	realFeedParser := gofeed.NewParser()
	os.Exit(run(os.Args, realFeedsIO, realFeedParser, os.Stdout, os.Stderr))
}

func run(args []string, feedsIO FeedsIO, feedFetcher FeedFetcher, stdout, stderr io.Writer) int {

	log := setupLogger()

	user_id := "rkladko@gmail.com"
	user_hash := GetSHA256(user_id)
	log.Info("user is ready", "id", user_id, "hash", user_hash)

	userFeedsFile, err := feedsIO.GetFeedsFile(user_hash)
	if err != nil {
		log.Error(err.Error())
		 return E_GET_FEED_FILE
	}
	log.Info("user feed file", "path", userFeedsFile)

	feeds, err := feedsIO.LoadFeeds(userFeedsFile)
	if err != nil {
		log.Error(err.Error())
		return E_READ_FEED_FILE
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sigChan:
			log.Info("received signal, initiating shutdown...", "signal", sig)
			cancel()
		case <-ctx.Done():
		}
	}()

	g, childCtx := errgroup.WithContext(ctx)

	sem := make(chan struct{}, maxConcurrentFeeds)

	for _, userFeed := range feeds.Items {

		feed := userFeed

		select {
		case sem <- struct{}{}:
		case <-childCtx.Done():
			log.Info("context cancelled before processing feed", "url", feed.Url)
			continue
		}

		g.Go(func() error {

			defer func() {
				<-sem // release "slot" after goroutine ends
			}()

			err = getUpdates(childCtx, feedFetcher, feed, log) 

			if err != nil {
				if errors.Is(err, context.Canceled) {
					log.Info("feed processing stopped due to cancellation", "url", feed.Url)
				} else {
					log.Error("failed to get updates for feed", "url", feed.Url, "error", err)
				}
				return err // errgroup.Group прекратит работу, если получит первую не nil ошибку
			}

			return nil
		})
	}

	log.Info("waiting for all feed updates to complete...")

	if err := g.Wait(); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Info("feed update process was cancelled.")
			return 0
		} else {
			log.Error("one or more feed updates failed", "error", err)
			return E_CONCURRENT_FAILURE
		}
	} else {
		log.Info("all feed updates completed successfully")
	}

	log.Info("saving updates to", "path", userFeedsFile)

	err = feedsIO.SaveUpdates(feeds, userFeedsFile)
	if err != nil {
		log.Error(err.Error())
		return E_UPDATE_FEED_FILE
	}

	log.Info("updates saved to", "path", userFeedsFile)

	log.Info("session done")

	return 0
}

func getUpdates(ctx context.Context, feedParser FeedFetcher, userFeed *Feed, log *slog.Logger) error {

	log.Info("processing feed", "url", userFeed.Url, "updated", userFeed.Updated)

	remoteFeed, err := feedParser.ParseURLWithContext(userFeed.Url, ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Info("feed processing cancelled", "url", userFeed.Url)
			return err
		}
		return err
	}

	if remoteFeed.Updated != userFeed.Updated {
		log.Info("got updates", "count", len(remoteFeed.Items))
		userFeed.Updated = remoteFeed.Updated
		newFeeds := 0

		for _, remoteItem := range remoteFeed.Items {
			select {
			case <-ctx.Done():
				log.Info("context cancelled while processing items, stopping early", "url", userFeed.Url)
				return ctx.Err()
			default:
				if _, exists := userFeed.UnprocessedGUID[remoteItem.GUID]; !exists {
					log.Info("new post", "guid", remoteItem.GUID, "title", firstNRunes(remoteItem.Title, 64), "updated", remoteItem.Updated)
					userFeed.UnprocessedGUID[remoteItem.GUID] = struct{}{}
					userFeed.UnprocessedItems = append(userFeed.UnprocessedItems, &UnprocessedItem{
						GUID: remoteItem.GUID,
						URL:  remoteItem.Link,
					})
					newFeeds++
				}
			}
		}
		log.Info("total new posts", "count", newFeeds)
	} else {
		log.Info("no new items")
	}
	return nil
}
