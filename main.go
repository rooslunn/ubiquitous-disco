package main

import (
	"encoding/json"
	"fmt"
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
	UnproccssedSet   string             `json:"unprocessed_set"`
	UnprocessedItems []*UnprocessedItem `json:"unprocessed_items"`
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

	for _, userFeed := range feeds.Items {

		fmt.Println("Feed info:")
		fmt.Printf("::Hash: %s, Updated: %s, URL: %s\n", userFeed.Hash, userFeed.Updated, userFeed.Url)

		feedParser := gofeed.NewParser()
		remoteFeed, err := feedParser.ParseURL(userFeed.Url)
		if err != nil {
			fmt.Println(err)
			continue
		}

		if remoteFeed.Updated != userFeed.Updated {

			fmt.Println("Received feeds count: ", len(remoteFeed.Items))

			userFeed.Updated = remoteFeed.Updated
			newFeeds := 0

			fmt.Println("New Items:")

			for i, remoteItem := range remoteFeed.Items {
				if !strings.Contains(userFeed.UnproccssedSet, remoteItem.GUID) {
					fmt.Printf("%d. GUID: %s, Title: %s, Date: %s\n", i, remoteItem.GUID, firstNRunes(remoteItem.Title, 64), remoteItem.Updated)
					userFeed.UnproccssedSet += "," + remoteItem.GUID
					userFeed.UnprocessedItems = append(userFeed.UnprocessedItems, &UnprocessedItem{
						GUID: remoteItem.GUID,
						URL:  remoteItem.Link,
					})
					newFeeds++
				}
			}
			fmt.Println("New feeds receieved: ", newFeeds)
		} else {
			fmt.Println("No new items")
		}

	}

	// save json
	file, err = os.Create(userFeedsFile)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(feeds); err != nil {
		panic(err)
	}

}
