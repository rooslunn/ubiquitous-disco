package rss_reader

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
)

var (
	ErrSHA256IncorrectLen = errors.New("user hash is too short")
)

func GetSHA256(name string) string {
	h := sha256.New()
	h.Write([]byte(name))
	hashInBytes := h.Sum(nil)
	hashInString := hex.EncodeToString(hashInBytes)
	return hashInString
}

func firstNRunes(s string, n int) string {
	runes := []rune(s)
	if n >= len(runes) {
		return s
	}
	return string(runes[:n])
}

func setupLogger(stdout io.Writer) *slog.Logger {
	// return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	return slog.New(slog.NewTextHandler(stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func deepCopy(originalMap UnrpocessedGUIDSet) UnrpocessedGUIDSet {
	deepCopyMap := make(UnrpocessedGUIDSet, len(originalMap))
	for key, value := range originalMap {
		deepCopyMap[key] = value
	}
	return deepCopyMap
}
