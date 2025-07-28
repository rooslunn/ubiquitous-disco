package rss_reader

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"syscall"
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

var TestAppArgs = []string{"rss_reader", "rkladko@gmail.com"}

func TestRunFunction(t *testing.T) {
	// buffs for saving output
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	// Scenario 1: Successful execution
	t.Run("Success", func(t *testing.T) {
		// Создаем мок FeedsIO для этого теста
		mockFeedsIO := &MockFeedsIO{
			GetFeedsFileFunc: func(userHash string) (string, error) {
				// success file
				return "test_feeds.json", nil
			},
			LoadFeedsFunc: func(userFeedsFile string) (Feeds, error) {
				// mock success load
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
				// success save
				return nil
			},
		}

		// Mock for FeedFetcher (для getUpdates)
		mockFeedFetcher := &MockGofeedParser{
			ParseURLWithContextFunc: func(feedURL string, ctx context.Context, ) (*gofeed.Feed, error) {
				return &gofeed.Feed{
					Updated: time.Now().Format(time.RFC3339),
					Items:   []*gofeed.Item{{GUID: "new_guid", Title: "New Post", Link: "http://example.com/new"}},
				}, nil
			},
		}

		// run with Mocks
		exitCode := run(TestAppArgs, mockFeedsIO, mockFeedFetcher, &stdoutBuf)

		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d. Stderr: %s", exitCode, stderrBuf.String())
		}
		output := stdoutBuf.String()
		if !strings.Contains(output, "session done") {
			t.Errorf("Expected 'session done' in output, got:\n%s", output)
		}
	})

	// Scenario 2: Error during GetFeedsFile (используем mockFeedsIO)
	t.Run("ErrorGetFeedsFile", func(t *testing.T) {
		stdoutBuf.Reset()
		stderrBuf.Reset()

		mockFeedsIO := &MockFeedsIO{
			GetFeedsFileFunc: func(userHash string) (string, error) {
				return "", errors.New("simulated GetFeedsFile error")
			},
			LoadFeedsFunc:   func(userFeedsFile string) (Feeds, error) { return Feeds{}, nil }, // Не будет вызван
			SaveUpdatesFunc: func(feeds Feeds, userFeedsFile string) error { return nil },     // Не будет вызван
		}
		mockFeedFetcher := &MockGofeedParser{} // Мок не важен для этого сценария

		exitCode := run(TestAppArgs, mockFeedsIO, mockFeedFetcher, &stdoutBuf)

		if exitCode != E_GET_FEED_FILE {
			t.Errorf("Expected exit code %d, got %d", E_GET_FEED_FILE, exitCode)
		}
		output := stdoutBuf.String()
		if !strings.Contains(output, "simulated GetFeedsFile error") {
			t.Errorf("Expected error message in output, got:\n%s", output)
		}
	})

	// Scenario 3: Error during loadFeeds (используем mockFeedsIO)
	t.Run("ErrorLoadFeeds", func(t *testing.T) {
		stdoutBuf.Reset()
		stderrBuf.Reset()

		mockFeedsIO := &MockFeedsIO{
			GetFeedsFileFunc: func(userHash string) (string, error) { return "test.json", nil },
			LoadFeedsFunc: func(userFeedsFile string) (Feeds, error) {
				return Feeds{}, errors.New("simulated loadFeeds error")
			},
			SaveUpdatesFunc: func(feeds Feeds, userFeedsFile string) error { return nil },
		}
		mockFeedFetcher := &MockGofeedParser{}

		exitCode := run(TestAppArgs, mockFeedsIO, mockFeedFetcher, &stdoutBuf)

		if exitCode != E_READ_FEED_FILE {
			t.Errorf("Expected exit code %d, got %d", E_READ_FEED_FILE, exitCode)
		}
		output := stdoutBuf.String()
		if !strings.Contains(output, "simulated loadFeeds error") {
			t.Errorf("Expected error message in output, got:\n%s", output)
		}
	})

	// Scenario 4: Error during getUpdates (один из фидов)
	t.Run("ErrorGetUpdates", func(t *testing.T) {
		stdoutBuf.Reset()
		stderrBuf.Reset()

		mockFeedsIO := &MockFeedsIO{
			GetFeedsFileFunc: func(userHash string) (string, error) { return "test.json", nil },
			LoadFeedsFunc: func(userFeedsFile string) (Feeds, error) {
				return Feeds{
					Items: []*Feed{
						{Url: "http://example.com/feed1.xml"}, // Этот фид вызовет ошибку
						{Url: "http://example.com/feed2.xml"},
					},
				}, nil
			},
			SaveUpdatesFunc: func(feeds Feeds, userFeedsFile string) error { return nil },
		}

		mockFeedFetcher := &MockGofeedParser{
			ParseURLWithContextFunc: func(feedURL string, ctx context.Context, ) (*gofeed.Feed, error) {
				if strings.Contains(feedURL, "feed1.xml") {
					return nil, errors.New("simulated getUpdates error for feed1")
				}
				return &gofeed.Feed{Updated: time.Now().Format(time.RFC3339)}, nil
			},
		}

		exitCode := run(TestAppArgs, mockFeedsIO, mockFeedFetcher, &stdoutBuf)

		if exitCode != E_CONCURRENT_FAILURE {
			t.Errorf("Expected exit code %d, got %d", E_CONCURRENT_FAILURE, exitCode)
		}
		output := stdoutBuf.String()
		if !strings.Contains(output, "simulated getUpdates error for feed1") {
			t.Errorf("Expected error message for getUpdates, got:\n%s", output)
		}
	})

	// Scenario 5: Error during saveUpdates (используем mockFeedsIO)
	t.Run("ErrorSaveUpdates", func(t *testing.T) {
		stdoutBuf.Reset()
		stderrBuf.Reset()

		mockFeedsIO := &MockFeedsIO{
			GetFeedsFileFunc: func(userHash string) (string, error) { return "test.json", nil },
			LoadFeedsFunc: func(userFeedsFile string) (Feeds, error) {
				return Feeds{Items: []*Feed{{Url: "http://example.com/feed1.xml"}}}, nil
			},
			SaveUpdatesFunc: func(feeds Feeds, userFeedsFile string) error {
				return errors.New("simulated saveUpdates error")
			},
		}
		mockFeedFetcher := &MockGofeedParser{
			ParseURLWithContextFunc: func(feedURL string, ctx context.Context, ) (*gofeed.Feed, error) {
				return &gofeed.Feed{Updated: time.Now().Format(time.RFC3339)}, nil
			},
		}

		exitCode := run(TestAppArgs, mockFeedsIO, mockFeedFetcher, &stdoutBuf)

		if exitCode != E_UPDATE_FEED_FILE {
			t.Errorf("Expected exit code %d, got %d", E_UPDATE_FEED_FILE, exitCode)
		}
		output := stdoutBuf.String()
		if !strings.Contains(output, "simulated saveUpdates error") {
			t.Errorf("Expected error message for saveUpdates, got:\n%s", output)
		}
	})

	// Scenario 6: Signal handling (simulating SIGTERM)
	t.Run("SignalHandling", func(t *testing.T) {
		stdoutBuf.Reset()
		stderrBuf.Reset()

		mockFeedsIO := &MockFeedsIO{
			GetFeedsFileFunc: func(userHash string) (string, error) { return "test.json", nil },
			LoadFeedsFunc: func(userFeedsFile string) (Feeds, error) {
				// Возвращаем много фидов, чтобы было что прерывать
				testFeedCount := 100
				items := make([]*Feed, testFeedCount)
				for i := range testFeedCount {
					items[i] = &Feed{Url: "http://example.com/slow_feed_" + string(rune('A'+i)) + ".xml", Updated: "2024-01-01T00:00:00Z"}
				}
				return Feeds{Items: items}, nil
			},
			SaveUpdatesFunc: func(feeds Feeds, userFeedsFile string) error {
				t.Error("SaveUpdates should not be called on graceful shutdown") // Если вызывается, это ошибка
				return nil
			},
		}

		mockFeedFetcher := &MockGofeedParser{
			ParseURLWithContextFunc: func(feedURL string, ctx context.Context, ) (*gofeed.Feed, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err() // Возвращаем ошибку отмены
				case <-time.After(50 * time.Millisecond): // Каждая загрузка занимает 50ms
					return &gofeed.Feed{Updated: time.Now().Format(time.RFC3339)}, nil
				}
			},
		}

		exitCodeChan := make(chan int, 1)
		go func() {
			exitCodeChan <- run(TestAppArgs, mockFeedsIO, mockFeedFetcher, &stdoutBuf)
		}()

		time.Sleep(100 * time.Millisecond) // Даем run запуститься и начать обработку

		syscall.Kill(os.Getpid(), syscall.SIGTERM) // Отправляем сигнал SIGTERM

		exitCode := <-exitCodeChan

		if exitCode != 0 {
			t.Errorf("Expected exit code 0 on signal, got %d", exitCode)
		}

		output := stdoutBuf.String()

		if !strings.Contains(output,  LOG_INFO_GRACEFUL_SHUTDOWN) {
			t.Errorf("Expected signal message in output, got:\n%s", output)
		}
		if !strings.Contains(output, LOG_INFO_UPDATE_CANCELLED) {
			t.Errorf("Expected cancellation message in output, got:\n%s", output)
		}
		// Проверяем, что сохранение не произошло
		if strings.Contains(output, LOG_INFO_SAVING_UPDATES) {
			t.Errorf("Expected 'saving updates to' NOT to be in output on cancellation, but it was:\n%s", output)
		}
	})
}