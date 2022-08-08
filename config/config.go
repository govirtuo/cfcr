package config

import (
	"fmt"
	"os"

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

func ParseConfig(file string) (*Config, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
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
