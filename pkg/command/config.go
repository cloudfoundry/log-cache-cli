package command

import (
	"io"
	"os"
	"path/filepath"

	envstruct "code.cloudfoundry.org/go-envstruct"
	homedir "github.com/mitchellh/go-homedir"
	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	Addr     string `yaml:"addr" env:"LOG_CACHE_ADDR"`
	SkipAuth bool   `yaml:"skip_auth" env:"LOG_CACHE_SKIP_AUTH"`
}

// BuildConfig reads in config file and ENV variables if set and returns a
// config object.
func BuildConfig() (Config, error) {
	home, err := homedir.Dir()
	if err != nil {
		return Config{}, err
	}

	confPath := filepath.Join(home, ".lc.yml")
	f, err := os.OpenFile(confPath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	var conf Config
	err = dec.Decode(&conf)
	if err != nil && err != io.EOF {
		return Config{}, err
	}

	err = envstruct.Load(&conf)
	if err != nil {
		return Config{}, err
	}

	return conf, nil
}
