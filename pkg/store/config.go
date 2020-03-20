package store

// TODO: this is next so we can start recording stuff.

type Config interface {
	TODO()
}

const (
	filename = ".bujo"
)

func LoadConfig() (Config, error) {
	// Walk the file tree from here backwards looking for a .bujo file.

	return &fileConfig{}, nil
}

type fileConfig struct {
}

func (f *fileConfig) TODO() {
	// TODO
}
