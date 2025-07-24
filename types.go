package rss_reader

import "github.com/mmcdole/gofeed"

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
}