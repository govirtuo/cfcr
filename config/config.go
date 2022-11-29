package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

type Config struct {
	LogLevel string `yaml:"log_level"`
	Auth     struct {
		Cloudflare struct {
			Email string `yaml:"email"`
			Key   string `yaml:"key"`
		} `yaml:"cloudflare"`
		OVH struct {
			AppKey      string `yaml:"app_key"`
			AppSecret   string `yaml:"app_secret"`
			ConsumerKey string `yaml:"consumer_key"`
		} `yaml:"ovh"`
	} `yaml:"auth"`
	Checks struct {
		BaseDomain string   `yaml:"base_domain"`
		Frequency  string   `yaml:"frequency"`
		Domains    []string `yaml:"domains"`
	} `yaml:"checks"`
	Metrics struct {
		Enabled bool `yaml:"enabled"`
		Server  struct {
			Address string `yaml:"address"`
			Port    string `yaml:"port"`
		} `yaml:"server"`
	} `yaml:"metrics"`
}

var validFrequencies = [5]string{
	"debug",
	"hourly",
	"daily",
	"weekly",
	"monthly",
}

func GetConfigFiles(l zerolog.Logger, path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if !fileInfo.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", path)
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	// data contains all the bytes that are read across all the configuration files
	var data []byte
	for _, file := range files {
		l.Info().Msgf("configuration file found: %s", file.Name())
		local, err := os.ReadFile(filepath.Join(path, file.Name()))
		if err != nil {
			return nil, err
		}

		// adding an extra line return is necessary if the file does not end by one.
		// Otherwise, we will end with lines such as: abc: deftoto: foo instead of:
		// abc: def
		// toto: foo
		local = append(local, '\n')
		data = append(data, local...)
	}

	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}

	return &c, nil
}

func (c Config) Validate() error {
	// parse and set the log level
	l, err := zerolog.ParseLevel(c.LogLevel)
	if err != nil {
		return err
	}
	zerolog.SetGlobalLevel(l)

	if c.Auth.Cloudflare.Email == "" {
		return errors.New("cloudflare configuration is incomplete: missing .auth.cloudflare.email field")
	}
	if c.Auth.Cloudflare.Key == "" {
		return errors.New("cloudflare configuration is incomplete: missing .auth.cloudflare.key field")
	}

	// parse and validate given frequency
	if !isFreqValid(c.Checks.Frequency) {
		return fmt.Errorf("frequency %s is not a valid one", c.Checks.Frequency)
	}
	return nil
}

func isFreqValid(f string) bool {
	for _, ff := range validFrequencies {
		if f == ff {
			return true
		}
	}
	return false
}
