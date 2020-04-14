package store

import (
	"github.com/spf13/viper"
	"log"
	"os"
)

// TODO: this is next so we can start recording stuff.

type Config interface {
	BasePath() string
}

func LoadConfig() (Config, error) {
	// Walk the file tree from here backwards looking for a .bujo file.
	viper.SetDefault("path", "~/.bujo.db") // TODO: we might want to default this to like ~/.bujo.db
	viper.SetConfigName(".bujo")           // .yaml is implicit
	viper.SetEnvPrefix("BUJO")
	viper.AutomaticEnv()

	if override := os.Getenv("BUJO_CONFIG_PATH"); override != "" {
		viper.AddConfigPath(override)
	}

	viper.AddConfigPath("./")

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
