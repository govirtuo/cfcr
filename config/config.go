package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Auth struct {
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
	}
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

// TODO
func (c Config) Validate() error {
	return nil
}
