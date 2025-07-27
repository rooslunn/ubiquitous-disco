package rss_reader

import (
	"context"

	"github.com/mmcdole/gofeed"
)

type Feeds struct {
	Version string  `json:"version"`
	Items   []*Feed `json:"items"`
}

type UnrpocessedGUIDSet map[string]struct{}

type Feed struct {
	Type             string             `json:"type"`
	Hash             string             `json:"hash"`
	Url              string             `json:"url"`
	Updated          string             `json:"updated"`
	UnprocessedGUID  UnrpocessedGUIDSet `json:"unprocessed_set"`
	UnprocessedItems []*UnprocessedItem `json:"unprocessed_items"`
}

type UnprocessedItem struct {
	URL  string `json:"url"`
	GUID string `json:"guid"`
}

type FeedFetcher interface {
	ParseURL(feedURL string) (*gofeed.Feed, error)
	ParseURLWithContext(feedURL string, ctx context.Context) (*gofeed.Feed, error)
}

// var EMPTY_STRUCT = struct{}{}

type MockGofeedParser struct {
	ParseURLFunc func(feedURL string) (*gofeed.Feed, error)
	ParseURLWithContextFunc func(feedURL string, ctx context.Context) (*gofeed.Feed, error)
}

func (m *MockGofeedParser) ParseURL(feedURL string) (*gofeed.Feed, error) {
	if m.ParseURLFunc != nil {
		return m.ParseURLFunc(feedURL)
	}
	return &gofeed.Feed{}, nil
}
func (m *MockGofeedParser) ParseURLWithContext(feedURL string, ctx context.Context ) (*gofeed.Feed, error) {
	if m.ParseURLWithContextFunc != nil {
		return m.ParseURLWithContextFunc(feedURL, ctx)
	}
	return &gofeed.Feed{}, nil // Default
}