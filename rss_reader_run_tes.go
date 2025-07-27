package rss_reader

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

type MockFeedsIO struct {
	GetFeedsFileFunc func(userHash string) (string, error)
	LoadFeedsFunc    func(userFeedsFile string) (Feeds, error)
	SaveUpdatesFunc  func(feeds Feeds, userFeedsFile string) error
}

// Реализация методов интерфейса FeedsIO для мока
func (m *MockFeedsIO) GetFeedsFile(userHash string) (string, error) {
	if m.GetFeedsFileFunc != nil {
		return m.GetFeedsFileFunc(userHash)
	}
	return "mock_feeds_file.json", nil // Поведение по умолчанию
}

func (m *MockFeedsIO) LoadFeeds(userFeedsFile string) (Feeds, error) {
	if m.LoadFeedsFunc != nil {
		return m.LoadFeedsFunc(userFeedsFile)
	}
	// Поведение по умолчанию: вернуть пустые фиды
	return Feeds{}, nil
}

func (m *MockFeedsIO) SaveUpdates(feeds Feeds, userFeedsFile string) error {
	if m.SaveUpdatesFunc != nil {
		return m.SaveUpdatesFunc(feeds, userFeedsFile)
	}
	return nil // Поведение по умолчанию: без ошибок
}

// TestRunFunction содержит тесты для функции run
func TestRunFunction(t *testing.T) {
	// Создаем буфер для захвата вывода stdout (логов)
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	// Scenario 1: Successful execution
	t.Run("Success", func(t *testing.T) {
		// Создаем мок FeedsIO для этого теста
		mockFeedsIO := &MockFeedsIO{
			GetFeedsFileFunc: func(userHash string) (string, error) {
				return "test_feeds.json", nil // Имитируем успешный путь
			},
			LoadFeedsFunc: func(userFeedsFile string) (Feeds, error) {
				// Имитируем загрузку фидов
				return Feeds{
					Items: []*Feed{
						{
							Url: "http://example.com/feed1.xml", 
							Updated: "2024-01-01T00:00:00Z", 
							UnprocessedGUID: UnrpocessedGUIDSet{"guid1": {}}, 
							UnprocessedItems: []*UnprocessedItem{{GUID: "guid1", URL: "url1"}},
						},
					},
				}, nil
			},
			SaveUpdatesFunc: func(feeds Feeds, userFeedsFile string) error {
				// Имитируем успешное сохранение
				return nil
			},
		}

		// Создаем мок FeedFetcher (для getUpdates)
		mockFeedFetcher := &MockGofeedParser{
			ParseURLWithContextFunc: func(feedURL string, ctx context.Context, ) (*gofeed.Feed, error) {
				return &gofeed.Feed{
					Updated: time.Now().Format(time.RFC3339),
					Items:   []*gofeed.Item{{GUID: "new_guid", Title: "New Post", Link: "http://example.com/new"}},
				}, nil
			},
		}

		// Вызываем run с моками
		exitCode := run([]string{"app_name"}, mockFeedsIO, mockFeedFetcher, &stdoutBuf, &stderrBuf)

		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d. Stderr: %s", exitCode, stderrBuf.String())
		}
		output := stdoutBuf.String()
		if !strings.Contains(output, "session done") {
			t.Errorf("Expected 'session done' in output, got:\n%s", output)
		}
	})
}