package rss_reader

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
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

func GetFeedsFile(hash string) (string, error) {
	if len(hash) < 64 {
		return "", ErrSHA256IncorrectLen
	}

	subFolder := hash[:2]
	userFile := hash[2:]

	file := filepath.Join(SERVICE_DIR, subFolder, userFile+".json")
	_, err := os.Stat(file)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return "", os.ErrNotExist
	}

	return file, nil
}

func firstNRunes(s string, n int) string {
	runes := []rune(s)
	if n >= len(runes) {
		return s
	}
	return string(runes[:n])
}

func setupLogger() *slog.Logger {
	// return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func deepCopy(originalMap UnrpocessedGUIDSet) UnrpocessedGUIDSet {
	deepCopyMap := make(UnrpocessedGUIDSet, len(originalMap))
	for key, value := range originalMap {
		deepCopyMap[key] = value
	}
	return deepCopyMap
}
