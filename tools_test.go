package rss_reader

import (
	"path/filepath"
	"testing"
)

func Test_firstNRunes(t *testing.T) {
	type args struct {
		s string
		n int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"test 1", args{"Correct", 3}, "Cor"},
		{"test 2", args{"Correct", 7}, "Correct"},
		{"test 3", args{"Correct", 13}, "Correct"},
		{"test 4", args{"Correct", 0}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstNRunes(tt.args.s, tt.args.n); got != tt.want {
				t.Errorf("firstNRunes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSHA256(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"test 1", args{"rkladko@gmail.com"}, "c1a2c0ab377be0752dde1a71f86fc71cc676574619c17f91e657b233590fe3e3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetSHA256(tt.args.name); got != tt.want {
				t.Errorf("GetSHA256() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetFeedsFile(t *testing.T) {
	type args struct {
		hash string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"test 1: correct", args{"c1a2c0ab377be0752dde1a71f86fc71cc676574619c17f91e657b233590fe3e3"}, filepath.Join(SERVICE_DIR,"c1/a2c0ab377be0752dde1a71f86fc71cc676574619c17f91e657b233590fe3e3.json"), false },
		{"test 2: hash len incorrect", args{"a2c0ab377be0752dde1a71f86fc71cc676574619c17f91e657b233590fe3e"}, "", true},
		{"test 3: file doesn't exist", args{"c132c0ab377be0752dde1a71f86fc71cc676574619c17f91e657b233590fe3e3"}, "", true },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feedsIO := &RealFeedsIO{}
			got, err := feedsIO.GetFeedsFile(tt.args.hash)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetFeedsFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetFeedsFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
