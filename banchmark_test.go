package rss_reader

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

type BenchmarkFeedsIO struct{}

func (b *BenchmarkFeedsIO) GetFeedsFile(userHash string) (string, error) {
	return "mock.json", nil
}

func (b *BenchmarkFeedsIO) LoadFeeds(userFeedsFile string) (Feeds, error) {
	feeds := Feeds{Items: make([]*Feed, 1000)}
	for i := range feeds.Items {
		feeds.Items[i] = &Feed{
			Url:              fmt.Sprintf("http://example.com/feed%d.xml", i),
			Updated:          time.Now().Format(time.RFC3339),
			UnprocessedGUID:  UnrpocessedGUIDSet{}, // Empty slice
			UnprocessedItems: []*UnprocessedItem{},
		}
	}
	return feeds, nil
}

func (b *BenchmarkFeedsIO) SaveUpdates(feeds Feeds, userFeedsFile string) error {
	return nil
}

type BenchmarkFeedFetcher struct{}

func (b *BenchmarkFeedFetcher) ParseURLWithContext(feedURL string, ctx context.Context) (*gofeed.Feed, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(10 * time.Millisecond): // Имитация сетевого запроса
		return &gofeed.Feed{
			Updated: time.Now().Format(time.RFC3339),
			Items:   []*gofeed.Item{{GUID: "new_guid", Title: "Test Post", Link: "http://example.com/test"}},
		}, nil
	}
}

func (m *BenchmarkFeedFetcher) ParseURL(feedURL string) (*gofeed.Feed, error) {
	return &gofeed.Feed{
		Updated: time.Now().Format(time.RFC3339),
		Items:   []*gofeed.Item{{GUID: "new_guid", Title: "Test Post", Link: "http://example.com/test"}},
	}, nil
}

func BenchmarkGetUpdates(b *testing.B) {
	mockFeedFetcher := &BenchmarkFeedFetcher{}

	userFeed := &Feed{
		Url:              "http://example.com/testfeed.xml",
		UnprocessedGUID:  UnrpocessedGUIDSet{}, // Empty slice
		UnprocessedItems: []*UnprocessedItem{},
	}

	log := setupLogger(io.Discard) // Выключаем логирование для чистоты бенчмарка

	b.ResetTimer()

	for b.Loop() {
		_ = getUpdates(context.Background(), mockFeedFetcher, userFeed, log)
	}
}

func BenchmarkRunFunction(b *testing.B) {

	mockFeedsIO := &BenchmarkFeedsIO{}
	mockFeedFetcher := &BenchmarkFeedFetcher{}

	devNull, _ := os.Open(os.DevNull)
	defer devNull.Close()

	for b.Loop() {
		_ = run(TestAppArgs, mockFeedsIO, mockFeedFetcher, devNull)
	}
}
