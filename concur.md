Отличная идея сделать `getUpdates` параллельным\! Это значительно ускорит обработку множества фидов. Добавление обработки сигналов обеспечит graceful shutdown приложения.

Для реализации параллелизма и корректной обработки сигналов мы будем использовать:

1.  **`goroutines`**: для запуска `getUpdates` параллельно.
2.  **`golang.org/x/sync/errgroup`**: для ожидания завершения всех горутин, управления контекстом и сбора ошибок. Это более мощная альтернатива `sync.WaitGroup` для таких сценариев.
3.  **`context.Context`**: для передачи сигнала отмены (`SIGTERM`, `SIGINT`) в горутины.
4.  **`os/signal`**: для перехвата системных сигналов.
5.  **Важно**: поскольку `getUpdates` модифицирует `userFeed` (который является указателем на структуру в `feeds.Items`), и каждая горутина будет работать с *разным* `userFeed`, **гонок данных на самих `Feed` структурах не будет**. Однако, если бы несколько горутин могли модифицировать *один и тот же* `Feed`, потребовалась бы синхронизация (например, `sync.Mutex`). В вашем случае `g.Wait()` гарантирует, что все обновления завершатся до вызова `saveUpdates`.

### Шаг 1: Обновление `getUpdates` и определение интерфейса

Сначала, давайте определим интерфейс для парсера фидов (`FeedFetcher`), чтобы сделать `getUpdates` более гибким и тестируемым. Затем обновим `getUpdates`, чтобы он использовал этот интерфейс и корректно обрабатывал отмену контекста.

```go
package main

import (
	"context" // Импортируем context
	"errors"
	"log/slog"
	"strings"

	"github.com/mmcdole/gofeed" // Убедитесь, что этот пакет установлен
)

// FeedFetcher определяет необходимые методы для получения и парсинга фида.
// gofeed.Parser реализует этот интерфейс.
type FeedFetcher interface {
	ParseURLWithContext(ctx context.Context, feedURL string) (*gofeed.Feed, error)
}

// MockGofeedParser (для использования в тестах)
// Эта структура должна быть определена в вашем тестовом файле (например, main_test.go)
// и должна быть доступна для getUpdates, если вы тестируете ее.
type MockGofeedParser struct {
	ParseURLWithContextFunc func(ctx context.Context, feedURL string) (*gofeed.Feed, error)
}

// ParseURLWithContext реализует метод интерфейса FeedFetcher для MockGofeedParser
func (m *MockGofeedParser) ParseURLWithContext(ctx context.Context, feedURL string) (*gofeed.Feed, error) {
	if m.ParseURLWithContextFunc != nil {
		return m.ParseURLWithContextFunc(ctx, feedURL)
	}
	// Поведение по умолчанию для мока, если функция не задана
	return &gofeed.Feed{}, nil
}

// getUpdates теперь принимает FeedFetcher интерфейс
func getUpdates(ctx context.Context, feedFetcher FeedFetcher, userFeed *Feed, log *slog.Logger) error {
	log.Info("processing feed", "url", userFeed.Url, "updated", userFeed.Updated)

	// Используем ParseURLWithContext для поддержки отмены
	remoteFeed, err := feedFetcher.ParseURLWithContext(ctx, userFeed.Url)
	if err != nil {
		// Проверяем, была ли ошибка вызвана отменой контекста
		if errors.Is(err, context.Canceled) {
			log.Info("feed processing cancelled", "url", userFeed.Url)
			return err // Важно вернуть ошибку контекста, чтобы errgroup ее обработал
		}
		return err // Другие ошибки
	}

	if remoteFeed.Updated != userFeed.Updated {
		log.Info("got updates", "count", len(remoteFeed.Items))
		userFeed.Updated = remoteFeed.Updated
		newFeeds := 0

		for _, remoteItem := range remoteFeed.Items {
			// Проверка на отмену контекста внутри цикла, если итерация может быть долгой
			select {
			case <-ctx.Done():
				log.Info("context cancelled while processing items, stopping early", "url", userFeed.Url)
				return ctx.Err() // Возвращаем ошибку контекста
			default:
				// Продолжаем
			}

            // Предполагается, что contains и firstNRunes определены в вашем пакете
			if !contains(userFeed.UnprocessedSet, remoteItem.GUID) {
				log.Info("new post", "guid", remoteItem.GUID, "title", firstNRunes(remoteItem.Title, 64), "updated", remoteItem.Updated)
				// Предполагается, что UnprocessedSet - это []string
				userFeed.UnprocessedSet = append(userFeed.UnprocessedSet, remoteItem.GUID)
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

// Ваши структуры Feed, UnprocessedItem и helper-функции (contains, firstNRunes)
// должны быть определены в вашем пакете. Пример:
type UnprocessedItem struct {
	GUID string
	URL  string
}

type Feeds struct {
	Items []*Feed // Предполагаем, что Feeds.Items - это слайс указателей на Feed
}

type Feed struct {
	Url              string
	Updated          string
	UnprocessedSet   []string // Предполагаем, что это слайс строк
	UnprocessedItems []*UnprocessedItem
}

func contains(s []string, e string) bool {
    for _, a := range s {
        if a == e {
            return true
        }
        // Если UnprocessedSet - это map[string]struct{}, то contains не нужен,
        // а проверка будет: if _, exists := userFeed.UnprocessedSet[remoteItem.GUID]; !exists
    }
    return false
}

func firstNRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
```

### Шаг 2: Обновление `main` функции для параллелизма и обработки сигналов

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall" // Для syscall.SIGTERM

	"github.com/mmcdole/gofeed"
	"golang.org/x/sync/errgroup" // Убедитесь, что установлен: go get golang.org/x/sync/errgroup
)

const (
	E_GET_FEED_FILE      = 1
	E_READ_FEED_FILE     = 2
	E_UPDATE_FEED_FILE   = 3
	E_CONCURRENT_FAILURE = 4 // Новый код ошибки для параллельных операций
)

// Предполагаемые заглушки для функций, которые не были предоставлены в вопросе
func setupLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

func GetSHA256(s string) string {
	// Просто заглушка
	return s + "_hash"
}

func GetFeedsFile(hash string) (string, error) {
	// Просто заглушка
	return "user_feeds_" + hash + ".json", nil
}

func loadFeeds(filePath string) (Feeds, error) {
	// Просто заглушка для загрузки фидов
	return Feeds{
		Items: []*Feed{
			{Url: "http://example.com/feed1.xml", Updated: "2024-01-01T00:00:00Z", UnprocessedSet: []string{"guidA"}, UnprocessedItems: []*UnprocessedItem{{GUID: "guidA", URL: "urlA"}}},
			{Url: "http://example.com/feed2.xml", Updated: "2024-01-01T00:00:00Z", UnprocessedSet: []string{"guidB"}, UnprocessedItems: []*UnprocessedItem{{GUID: "guidB", URL: "urlB"}}},
			{Url: "http://example.com/feed3.xml", Updated: "2024-01-01T00:00:00Z", UnprocessedSet: []string{"guidC"}, UnprocessedItems: []*UnprocessedItem{{GUID: "guidC", URL: "urlC"}}},
		},
	}, nil
}

func saveUpdates(feeds Feeds, filePath string) error {
	// Просто заглушка для сохранения фидов
	return nil
}

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

	// 1. Создание корневого контекста с отменой для обработки сигналов
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Гарантируем вызов cancel() при выходе из main

	// 2. Настройка обработчика системных сигналов (SIGINT, SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM) // Перехватываем Ctrl+C и SIGTERM

	go func() {
		select {
		case sig := <-sigChan:
			log.Info("received signal, initiating graceful shutdown...", "signal", sig)
			cancel() // Отменяем контекст, что прервет все запущенные горутины
		case <-ctx.Done():
			// Контекст уже отменен (возможно, другой причиной), ничего не делаем
		}
	}()

	// 3. Инициализация errgroup.Group с контекстом
	// errgroup.WithContext возвращает Group и дочерний контекст.
	// Если любая горутина вернет ошибку или корневой контекст будет отменен,
	// дочерний контекст также будет отменен.
	g, childCtx := errgroup.WithContext(ctx)

	// Создаем один парсер для всех горутин. gofeed.Parser является потокобезопасным.
	feedParser := gofeed.NewParser()

	// Опционально: Ограничение параллелизма
	// Если у вас очень много фидов и вы не хотите перегружать систему или API
	const maxConcurrentFeeds = 5 // Например, не более 5 одновременных загрузок
	sem := make(chan struct{}, maxConcurrentFeeds) // Буферизованный канал как семафор

	// 4. Запуск getUpdates в параллельных горутинах
	for _, userFeed := range feeds.Items {
		// Важно: захватываем переменную итерации в замыкание,
		// чтобы каждая горутина работала со своей собственной копией userFeed.
		// Поскольку userFeed - это *Feed, мы передаем указатель,
		// и каждая горутина будет модифицировать свой уникальный объект Feed.
		feed := userFeed // Создаем новую переменную в каждой итерации

		// Занимаем "слот" в семафоре перед запуском горутины
		select {
		case sem <- struct{}{}: // Пытаемся занять слот
			// Слот занят, запускаем горутину
		case <-childCtx.Done(): // Если контекст отменен до занятия слота
			log.Info("context cancelled before starting new feed processing", "url", feed.Url)
			// Пропускаем оставшиеся фиды, так как контекст отменен
			continue
		}

		g.Go(func() error {
			defer func() {
				<-sem // Освобождаем "слот" при завершении горутины (даже при панике или ошибке)
			}()

			// Вызываем getUpdates с дочерним контекстом
			err = getUpdates(childCtx, feedParser, feed, log)
			if err != nil {
				// Если ошибка - это отмена контекста, логируем ее как info,
				// иначе как error.
				if errors.Is(err, context.Canceled) {
					log.Info("feed processing stopped due to cancellation", "url", feed.Url)
				} else {
					log.Error("failed to get updates for feed", "url", feed.Url, "error", err)
				}
				// errgroup.Group прекратит ожидание и вернет первую не nil ошибку.
				// Важно вернуть ошибку, чтобы g.Wait() ее поймал.
				return err
			}
			return nil
		})
	}

	// 5. Ожидание завершения всех горутин
	log.Info("waiting for all feed updates to complete...")
	if err := g.Wait(); err != nil {
		// Если здесь есть ошибка, значит, одна из горутин вернула ошибку,
		// или контекст был отменен.
		if errors.Is(err, context.Canceled) {
			log.Info("feed update process was cancelled by signal.")
			os.Exit(0) // Завершаем программу без ошибки, если была отмена
		} else {
			log.Error("one or more feed updates failed during concurrent processing", "error", err)
			os.Exit(E_CONCURRENT_FAILURE) // Выходим с кодом ошибки
		}
	} else {
		log.Info("all feed updates completed successfully")
	}

	log.Info("saving updates to", "path", userFeedsFile)

	// saveUpdates будет вызван только после завершения всех горутин
	err = saveUpdates(feeds, userFeedsFile)
	if err != nil {
		log.Error("failed to save updates", "error", err)
		os.Exit(E_UPDATE_FEED_FILE)
	}

	log.Info("updates saved to", "path", userFeedsFile)
	log.Info("session done")
}
```

### Как это работает:

1.  **`context.WithCancel(context.Background())`**: Создает родительский контекст, который мы можем отменить.
2.  **`signal.Notify`**: Настраивает канал `sigChan` для получения системных сигналов `SIGINT` (Ctrl+C) и `SIGTERM`.
3.  **Горутина-слушатель сигналов**: Отдельная горутина ждет получения сигнала из `sigChan`. Как только сигнал получен, она вызывает `cancel()`, что приводит к отмене контекста `ctx`.
4.  **`errgroup.WithContext(ctx)`**: Создает `errgroup.Group` и дочерний контекст (`childCtx`). Этот `childCtx` автоматически отменяется, если `ctx` отменяется, или если любая горутина, запущенная через `g.Go()`, возвращает ошибку.
5.  **`g.Go(func() error { ... })`**: Запускает каждую операцию `getUpdates` в отдельной горутине. Важно, что `feed := userFeed` создает локальную копию указателя на `Feed` для каждой горутины, избегая проблем с замыканиями.
6.  **`sem := make(chan struct{}, maxConcurrentFeeds)`**: Это семафор. `sem <- struct{}{}` блокирует выполнение, пока в канале нет свободного места, тем самым ограничивая количество одновременно запущенных горутин. `defer func() { <-sem }()` гарантирует, что слот освободится, когда горутина завершится.
7.  **`feedFetcher.ParseURLWithContext(ctx, userFeed.Url)`**: `gofeed` имеет метод, который принимает контекст. Это позволяет ему прервать сетевые операции, если контекст отменяется (например, при получении `SIGTERM`).
8.  **Проверка `<-ctx.Done()`**: Внутри `getUpdates` и перед запуском каждой горутины добавлена проверка `select { case <-ctx.Done(): ... }`. Это позволяет горутинам быстро реагировать на отмену, даже если они не заблокированы сетевой операцией.
9.  **`g.Wait()`**: Блокирует `main` горутину до тех пор, пока все запущенные задачи не завершатся. Если какая-либо задача вернет ошибку, `g.Wait()` вернет эту ошибку, и вы можете соответствующим образом обработать сбой. Если контекст был отменен, `g.Wait()` вернет `context.Canceled`.
10. **Обработка ошибок `g.Wait()`**: Проверяем, является ли ошибка `context.Canceled`. Если да, это "мягкое" завершение, и мы можем выйти с кодом `0`. В противном случае это была настоящая ошибка в одной из горутин, и мы выходим с кодом `E_CONCURRENT_FAILURE`.

Это решение обеспечивает высокую производительность за счет параллелизма и надежность за счет обработки ошибок и сигналов.0