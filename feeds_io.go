package rss_reader

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type FeedsIO interface {
	GetFeedsFile(userHash string) (string, error)
	LoadFeeds(userFeedsFile string) (Feeds, error)
	SaveUpdates(feeds Feeds, userFeedsFile string) error
}

type RealFeedsIO struct {}

func (r *RealFeedsIO) GetFeedsFile(hash string) (string, error) {
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

func (r *RealFeedsIO) LoadFeeds(userFeedsFile string) (Feeds, error) {
	file, err := os.Open(userFeedsFile)
	if err != nil {
		return Feeds{}, err
	}
	defer file.Close()

	var feeds Feeds
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&feeds); err != nil {
		return Feeds{}, err
	}

	if feeds.Items == nil {
		return Feeds{}, ErrFeedInvalidJson
	}

	return feeds, nil

}
func (r *RealFeedsIO) SaveUpdates(feeds Feeds, userFeedsFile string) error {
	file, err := os.Create(userFeedsFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(feeds); err != nil {
		return err
	}

	return nil
}