package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mmcdole/gofeed"
)

type Feeds struct {
	Version     string  `json:"version"`
	Items       []*Feed `json:"items"`
}

type Feed struct {
	Type    string `json:"type"`
	Hash    string `json:"hash"`
	Url     string `json:"url"`
	Updated string `json:"updated"`
	UnproccessedSet string
}

const (
	SERVICE_DIR = "./.sputnik"
)

func main() {

	user := GetSHA256("rkladko@gmail.com")

	userFeedsFile, err := GetFeedsFile(user)
	if err != nil {
		panic(err)
	}

	file, err := os.Open(userFeedsFile)
	if err != nil {
		panic(err)
	}

	var feeds Feeds
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&feeds); err != nil {
		panic(err)
	}
	file.Close()

	for _, f := range feeds.Items {

		fmt.Printf("Hash: %s, Updated: %s, URL: %s\n", f.Hash, f.Updated, f.Url)

		fp := gofeed.NewParser()
		feed, err := fp.ParseURL(f.Url)
		if err != nil {
			fmt.Println(err)
			continue
		}

		if feed.Updated != f.Updated {
			fmt.Println("Feeds updated", len(feed.Items))
			f.Updated = feed.Updated
			for _, item := range feed.Items {
				fmt.Printf("Title: %s, Date: %s, GUID: %s\n", firstNRunes(item.Title, 64), item.Updated, item.GUID)
			}
		} else {
			fmt.Println("No new items")
		}

	}

	// save json
	file, err = os.Create("feeds.json")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(feeds); err != nil {
		panic(err)
	}

}

func firstNRunes(s string, n int) string {
	runes := []rune(s)
	if n >= len(runes) {
		return s
	}
	return string(runes[:n])
}
