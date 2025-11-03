// Package store exposes configuration loading and persistence interfaces.
package store

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

// Config describes the persistence base path configuration.
type Config interface {
	BasePath() string
}

// LoadConfig resolves configuration from disk/environment.
func LoadConfig() (Config, error) {
	homeDir, err := homedir.Dir()
	if err != nil {
		log.Printf("Couldn't detect home dir, using cwd: %s", err)
		homeDir = "."
	}
	// Walk the file tree from here backwards looking for a .bujo file.
	viper.SetDefault("path", filepath.Join(homeDir, ".bujo.db"))
	viper.SetConfigName(".bujo") // .yaml is implicit
	viper.SetEnvPrefix("BUJO")
	viper.AutomaticEnv()

	if override := os.Getenv("BUJO_CONFIG_PATH"); override != "" {
		viper.AddConfigPath(expandPath(override))
	}

	viper.AddConfigPath(homeDir)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Fatalf("error reading config file: %v", err)
			return nil, err
		}
	}

	path := expandPath(viper.GetString("path"))
	if strings.TrimSpace(path) == "" {
		path = filepath.Join(homeDir, ".bujo.db")
	}
	return &fileConfig{Path: path}, nil
}

type fileConfig struct {
	Path string `json:"path"`
}

func (f *fileConfig) BasePath() string {
	return f.Path
}

func expandPath(p string) string {
	if strings.TrimSpace(p) == "" {
		return p
	}
	expanded, err := homedir.Expand(p)
	if err != nil {
		return p
	}
	return expanded
}
