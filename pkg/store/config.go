package store

import (
	"log"
	"os"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

// TODO: this is next so we can start recording stuff.

type Config interface {
	BasePath() string
}

func LoadConfig() (Config, error) {
	homeDir, err := homedir.Dir()
	if err != nil {
		log.Printf("Couldn't detect home dir, using cwd: %s", err)
		homeDir = "."
	}
	// Walk the file tree from here backwards looking for a .bujo file.
	viper.SetDefault("path", homeDir+"/.bujo.db")
	viper.SetConfigName(".bujo") // .yaml is implicit
	viper.SetEnvPrefix("BUJO")
	viper.AutomaticEnv()

	if override := os.Getenv("BUJO_CONFIG_PATH"); override != "" {
		viper.AddConfigPath(override)
	}

	viper.AddConfigPath(homeDir)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Fatalf("error reading config file: %v", err)
			return nil, err
		}
	}

	return &fileConfig{Path: viper.GetString("path")}, nil
}

type fileConfig struct {
	Path string `json:"path"`
}

func (f *fileConfig) BasePath() string {
	return f.Path
}
