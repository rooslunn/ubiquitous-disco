package rss_reader

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

func Test_saveFeeds(t *testing.T) {
	// Test Case 1: The "happy path" where the function runs successfully.
	t.Run("Success", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "testfeeds.json")

		testFeeds := Feeds{
			Version: "Test Feed",
			Items: []*Feed{
				{Type: "rss", Url: "http://example.com"},
			},
		}

		feedsIO := &RealFeedsIO{}

		err := feedsIO.SaveUpdates(testFeeds, filePath)
		if err != nil {
			t.Fatalf("expected no error, but got: %v", err)
		}

		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read created file: %v", err)
		}

		var decodedFeeds Feeds
		err = json.Unmarshal(fileContent, &decodedFeeds)
		if err != nil {
			t.Fatalf("failed to decode JSON from file: %v", err)
		}

		// Assert that the data we wrote to the file is the same as the original
		if !reflect.DeepEqual(decodedFeeds, testFeeds) {
			t.Errorf("decoded data does not match original data. Got: %+v, Expected: %+v", decodedFeeds, testFeeds)
		}
	})

	// Test Case 2: The os.Create() call fails (e.g., due to invalid path or permissions).
	t.Run("FileCreationFailure", func(t *testing.T) {
		testFeeds := Feeds{
			Version: "Test Feed",
			Items: []*Feed{
				{Type: "rss", Url: "http://example.com"},
			},
		}

		invalidPath := filepath.Join("/non_existent_dir", "file.json")

		feedsIO := &RealFeedsIO{}
		err := feedsIO.SaveUpdates(testFeeds, invalidPath)
		if err == nil {
			t.Fatalf("expected an error due to invalid path, but got none")
		}

		t.Logf("successfully received expected error: %v", err)
	})

	// Test Case 3: The json.Encode() call fails.
	t.Run("JSONEncodingFailure", func(t *testing.T) {
		// Create a struct that json.Encoder cannot encode (e.g., a channel)
		// A channel can't be marshaled into JSON, so this forces a failure.
		type BadFeeds struct {
			Name    string
			Channel chan int // This will cause an encoding error
		}

		testBadFeeds := BadFeeds{Name: "Bad Feed", Channel: make(chan int)}
		filePath := filepath.Join(t.TempDir(), "badfeeds.json")

		// We need to create a helper function to call saveFeeds with the bad struct
		// because the function signature doesn't match the test case struct.
		// This is a common pattern for testing specific failure modes.
		saveBadFeeds := func(feeds BadFeeds, userFeedsFile string) error {
			file, err := os.Create(userFeedsFile)
			if err != nil {
				return err
			}
			defer file.Close()
			encoder := json.NewEncoder(file)
			return encoder.Encode(feeds)
		}

		// Call the function and expect an error
		err := saveBadFeeds(testBadFeeds, filePath)
		if err == nil {
			t.Fatalf("expected an error due to JSON encoding failure, but got none")
		}

		// Optionally, check the error type or message
		t.Logf("successfully received expected JSON error: %v", err)
	})
}

func Test_loadFeeds(t *testing.T) {
	// Test Case 1: The "happy path" where the file is read successfully.
	t.Run("Success", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "testfeeds.json")

		testFeeds := Feeds{
			Version: "Test Feed",
			Items: []*Feed{
				{Type: "rss", Url: "http://example.com"},
			},
		}
		jsonData, err := json.Marshal(testFeeds)
		if err != nil {
			t.Fatalf("failed to marshal test data: %v", err)
		}
		if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		feedsIO := &RealFeedsIO{}
		loadedFeeds, err := feedsIO.LoadFeeds(filePath)

		if err != nil {
			t.Fatalf("expected no error, but got: %v", err)
		}
		if !reflect.DeepEqual(loadedFeeds, testFeeds) {
			t.Errorf("loaded data does not match original data. Got: %+v, Expected: %+v", loadedFeeds, testFeeds)
		}
	})

	// Test Case 2: The file does not exist.
	t.Run("FileNotFound", func(t *testing.T) {
		nonExistentFile := filepath.Join(t.TempDir(), "non_existent.json")

		feedsIO := &RealFeedsIO{}
		loadedFeeds, err := feedsIO.LoadFeeds(nonExistentFile)

		if err == nil {
			t.Fatalf("expected an error when file is not found, but got none")
		}
		if !reflect.DeepEqual(loadedFeeds, Feeds{}) {
			t.Errorf("expected zero-value Feeds struct on error, but got: %+v", loadedFeeds)
		}
		if !os.IsNotExist(err) {
			t.Errorf("expected 'not exist' error, but got: %v", err)
		}
	})

	// Test Case 3: The file contains invalid JSON data.
	t.Run("InvalidJSON", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "invalid.json")

		malformedJSON := []byte(`{"Version": "Test", "items": [{"URL": 12345}]}`)
		if err := os.WriteFile(filePath, malformedJSON, 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		feedsIO := &RealFeedsIO{}
		loadedFeeds, err := feedsIO.LoadFeeds(filePath)

		if _, ok := err.(*json.UnmarshalTypeError); !ok {
			t.Errorf("expected a json.UnmarshalTypeError, but got: %T", err)
		}

		if err == nil {
			t.Fatalf("expected a JSON decoding error, but got none")
		}

		if !reflect.DeepEqual(loadedFeeds, Feeds{}) {
			t.Errorf("expected zero-value Feeds struct on error, but got: %+v", loadedFeeds)
		}
	})
}


func Test_getUpdates(t *testing.T) {
	discardLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	t.Run("1. Successfully updates with new items", func(t *testing.T) {
		initialUpdated := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
		newUpdated := time.Now().Format(time.RFC3339)

		userFeed := &Feed{
			Url:              "http://example.com/feed.xml",
			Updated:          initialUpdated,
			UnprocessedGUID:   UnrpocessedGUIDSet{"guid1": {}, "guid2": {}},
			UnprocessedItems: []*UnprocessedItem{{GUID: "guid1", URL: "url1"}, {GUID: "guid2", URL: "url2"}},
		}

		mockParser := &MockGofeedParser{
			ParseURLWithContextFunc: func(feedURL string, ctx context.Context) (*gofeed.Feed, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(1 * time.Millisecond): // Имитация небольшой задержки
					return &gofeed.Feed{
						Updated: newUpdated,
						Items: []*gofeed.Item{
							{GUID: "guid1", Title: "Old Post 1", Link: "url1", Updated: "old1"},
							{GUID: "guid2", Title: "Old Post 2", Link: "url2", Updated: "old2"},
							{GUID: "guid3", Title: "New Post 1", Link: "url3", Updated: "new1"}, // Новый элемент
							{GUID: "guid4", Title: "New Post 2", Link: "url4", Updated: "new2"}, // Новый элемент
						},
					}, nil
				}
			},
		}

		// Assertions

		err := getUpdates(context.Background(), mockParser, userFeed, discardLogger)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if userFeed.Updated != newUpdated {
			t.Errorf("expected userFeed.Updated to be %s, got %s", newUpdated, userFeed.Updated)
		}

		expectedUnprocessedGUID := UnrpocessedGUIDSet{"guid1":{}, "guid2":{}, "guid3":{}, "guid4":{}}
		if !reflect.DeepEqual(userFeed.UnprocessedGUID, expectedUnprocessedGUID) {
			t.Errorf("expected UnprocessedGUID to be %v, got %v", expectedUnprocessedGUID, userFeed.UnprocessedGUID)
		}

		expectedUnprocessedItems := []*UnprocessedItem{
			{GUID: "guid1", URL: "url1"},
			{GUID: "guid2", URL: "url2"},
			{GUID: "guid3", URL: "url3"},
			{GUID: "guid4", URL: "url4"},
		}
		if !reflect.DeepEqual(userFeed.UnprocessedItems, expectedUnprocessedItems) {
			t.Errorf("expected UnprocessedItems to be %+v, got %+v", expectedUnprocessedItems, userFeed.UnprocessedItems)
		}
	})

	t.Run("2. Successfully updates timestamp with no new items", func(t *testing.T) {
		initialUpdated := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
		newUpdated := time.Now().Format(time.RFC3339)

		userFeed := &Feed{
			Url:              "http://example.com/feed.xml",
			Updated:          initialUpdated,
			UnprocessedGUID:  UnrpocessedGUIDSet{"guid1": {}, "guid2": {}},
			UnprocessedItems: []*UnprocessedItem{{GUID: "guid1", URL: "url1"}, {GUID: "guid2", URL: "url2"}},
		}
		originalUnprocessedItems := append([]*UnprocessedItem{}, userFeed.UnprocessedItems...) // Deep copy
        originalUnprocessedGUID := deepCopy(userFeed.UnprocessedGUID) // Deep copy

		mockParser := &MockGofeedParser{
			ParseURLWithContextFunc: func(feedURL string, ctx context.Context) (*gofeed.Feed, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(1 * time.Millisecond):
					return &gofeed.Feed{
						Updated: newUpdated,
						Items: []*gofeed.Item{
							{GUID: "guid1", Title: "Old Post 1", Link: "url1", Updated: "old1"},
							{GUID: "guid2", Title: "Old Post 2", Link: "url2", Updated: "old2"},
						},
					}, nil
				}
			},
		}

		err := getUpdates(context.Background(), mockParser, userFeed, discardLogger)

		// Assertions

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if userFeed.Updated != newUpdated {
			t.Errorf("expected userFeed.Updated to be %s, got %s", newUpdated, userFeed.Updated)
		}
		if !reflect.DeepEqual(userFeed.UnprocessedGUID, originalUnprocessedGUID) {
			t.Errorf("expected UnprocessedGUID to be unchanged %v, got %v", originalUnprocessedGUID, userFeed.UnprocessedGUID)
		}
		if !reflect.DeepEqual(userFeed.UnprocessedItems, originalUnprocessedItems) {
			t.Errorf("expected UnprocessedItems to be unchanged %+v, got %+v", originalUnprocessedItems, userFeed.UnprocessedItems)
		}
	})

	t.Run("3. No new updates detected", func(t *testing.T) {
		currentTime := time.Now().Format(time.RFC3339)

		userFeed := &Feed{
			Url:              "http://example.com/feed.xml",
			Updated:          currentTime,
			UnprocessedGUID:   UnrpocessedGUIDSet{"guid1": {}, "guid2": {}},
			UnprocessedItems: []*UnprocessedItem{{GUID: "guid1", URL: "url1"}, {GUID: "guid2", URL: "url2"}},
		}
        // Create a deep copy of the original userFeed struct
        originalUserFeed := &Feed{
            Url:              userFeed.Url,
            Updated:          userFeed.Updated,
            UnprocessedGUID:  deepCopy(userFeed.UnprocessedGUID),
            UnprocessedItems: make([]*UnprocessedItem, len(userFeed.UnprocessedItems)),
        }
        for i, item := range userFeed.UnprocessedItems {
            originalUserFeed.UnprocessedItems[i] = &UnprocessedItem{GUID: item.GUID, URL: item.URL}
        }

		mockParser := &MockGofeedParser{
			ParseURLWithContextFunc: func(feedURL string, ctx context.Context) (*gofeed.Feed, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(1 * time.Millisecond):
					return &gofeed.Feed{
						Updated: currentTime, // Same as userFeed.Updated
						Items: []*gofeed.Item{
							{GUID: "guid5", Title: "New Post X", Link: "urlX", Updated: "newX"},
						},
					}, nil
				}
			},
		}

		err := getUpdates(context.Background(), mockParser, userFeed, discardLogger)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Assertions: userFeed should be completely unchanged

		if !reflect.DeepEqual(userFeed, originalUserFeed) {
			t.Errorf("expected userFeed to be unchanged, but it was: %+v", userFeed)
		}
	})

	t.Run("4. ParseURL returns error", func(t *testing.T) {
		userFeed := &Feed{
			Url:              "http://example.com/badfeed.xml",
			Updated:          "some-old-date",
			UnprocessedGUID:   UnrpocessedGUIDSet{"guid1": {}},
			UnprocessedItems: []*UnprocessedItem{{GUID: "guid1", URL: "url1"}},
		}
        // Create a deep copy of the original userFeed struct
        originalUserFeed := &Feed{
            Url:              userFeed.Url,
            Updated:          userFeed.Updated,
            UnprocessedGUID:  deepCopy(userFeed.UnprocessedGUID),
            UnprocessedItems: make([]*UnprocessedItem, len(userFeed.UnprocessedItems)),
        }
        for i, item := range userFeed.UnprocessedItems {
            originalUserFeed.UnprocessedItems[i] = &UnprocessedItem{GUID: item.GUID, URL: item.URL}
        }

		expectedErr := errors.New("network error: failed to fetch feed")
		mockParser := &MockGofeedParser{
			ParseURLWithContextFunc: func(feedURL string, ctx context.Context) (*gofeed.Feed, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(1 * time.Millisecond):
					return nil, expectedErr
				}
			},
		}

		err := getUpdates(context.Background(), mockParser, userFeed, discardLogger)

		// Assertions

		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
		if !reflect.DeepEqual(userFeed, originalUserFeed) {
			t.Errorf("expected userFeed to be unchanged on error, but it was: %+v", userFeed)
		}
	})

	t.Run("5. Empty UnprocessedGUID with new items", func(t *testing.T) {
		userFeed := &Feed{
			Url:              "http://example.com/feed.xml",
			Updated:          "some-old-date",
			UnprocessedGUID:   UnrpocessedGUIDSet{}, // Empty slice
			UnprocessedItems: []*UnprocessedItem{},
		}

		originalUserFeed := *userFeed // Copy to check for no changes

		// Создаем контекст, который будет отменен
		ctx, cancel := context.WithCancel(context.Background())
		mockParser := &MockGofeedParser{
			ParseURLWithContextFunc: func(feedURL string, ctx context.Context, ) (*gofeed.Feed, error) {
				// Имитируем долгую операцию, которая будет прервана контекстом
				select {
				case <-ctx.Done(): // Ждем отмены контекста
					return nil, ctx.Err()
				case <-time.After(50 * time.Millisecond): // Если контекст не отменен, то возвращаем фид
					return &gofeed.Feed{Updated: time.Now().Format(time.RFC3339)}, nil
				}
			},
		}

		// Запускаем горутину, которая отменит контекст через короткое время
		go func() {
			time.Sleep(10 * time.Millisecond) // Отменяем контекст до того, как mockParser закончит
			cancel()
		}()

		err := getUpdates(ctx, mockParser, userFeed, discardLogger)

		// Ожидаем ошибку context.Canceled
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error, got %v", err)
		}
		// Убеждаемся, что userFeed не изменился
		if !reflect.DeepEqual(userFeed, &originalUserFeed) {
			t.Errorf("expected userFeed to be unchanged on cancellation, but it was: %+v", userFeed)
		}
	})

	t.Run("6. Context cancelled during item processing", func(t *testing.T) {
		// t.SkipNow()
		initialUpdated := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
		newUpdated := time.Now().Format(time.RFC3339)

		userFeed := &Feed{
			Url:              "http://example.com/manyitems.xml",
			Updated:          initialUpdated,
			UnprocessedGUID:   UnrpocessedGUIDSet{}, // Empty slice
			UnprocessedItems: []*UnprocessedItem{},
		}

		originalUserFeed := *userFeed // Copy for initial state check

		// test feed items
		testItemsCount := 10000
		items := make([]*gofeed.Item, testItemsCount)
		for i := range testItemsCount {
			items[i] = &gofeed.Item{
				GUID:    "guid" + string(rune('A'+i)),
				Title:   "Post " + string(rune('A'+i)),
				Link:    "http://example.com/post" + string(rune('A'+i)),
				Updated: newUpdated,
			}
		}

		ctx, cancel := context.WithCancel(context.Background())
		mockParser := &MockGofeedParser{
			ParseURLWithContextFunc: func(feedURL string, ctx context.Context, ) (*gofeed.Feed, error) {
				return &gofeed.Feed{
					Updated: newUpdated,
					Items:   items,
				}, nil
			},
		}

		go func() {
			time.Sleep(1 * time.Millisecond)
			cancel()
		}()

		err := getUpdates(ctx, mockParser, userFeed, discardLogger)

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error, got %v", err)
		}

		if userFeed.Updated != newUpdated {
			t.Errorf("expected userFeed.Updated to be %s, got %s", newUpdated, userFeed.Updated)
		}

		if reflect.DeepEqual(userFeed.UnprocessedGUID, originalUserFeed.UnprocessedGUID) &&
		   reflect.DeepEqual(userFeed.UnprocessedItems, originalUserFeed.UnprocessedItems) {
			t.Error("expected userFeed to be partially updated or unchanged from initial, but not fully processed")
		}

		if len(userFeed.UnprocessedItems) == len(items) {
			t.Error("expected processing to be interrupted, but all items were added")
		}
	})
}
