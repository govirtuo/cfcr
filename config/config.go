package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Logging Logging `yaml:"logging"`
	Auth    struct {
		Cloudflare struct {
			Token string `yaml:"token"`
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

type Logging struct {
	Level         string `yaml:"level"`
	HumanReadable bool   `yaml:"human_readable"`
}

var validFrequencies = [5]string{
	"debug",
	"hourly",
	"daily",
	"weekly",
	"monthly",
}

// GetConfigFiles parses all the YAML files present in the path
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
		if !isYamlFile(file.Name()) {
			l.Debug().Msgf("file '%s' does not seem to be a YAML file, skipping its content",
				file.Name())
			continue
		}
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

// Validates check that the required fields are set within c
func (c Config) Validate() error {
	// parse and set the log level
	l, err := zerolog.ParseLevel(c.Logging.Level)
	if err != nil {
		return err
	}
	zerolog.SetGlobalLevel(l)

	if c.Auth.Cloudflare.Token == "" {
		return errors.New("cloudflare configuration is incomplete: missing .auth.cloudflare.token field")
	}

	// parse and validate given frequency
	if !isFreqValid(c.Checks.Frequency) {
		return fmt.Errorf("frequency %s is not a valid one", c.Checks.Frequency)
	}
	return nil
}

// isFreqValid checks that f is a supported frequency
func isFreqValid(f string) bool {
	for _, ff := range validFrequencies {
		if f == ff {
			return true
		}
	}
	return false
}

// isYamlFile checks if the filename ends with either .yaml or .yml
func isYamlFile(name string) bool {
	if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
		return true
	}
	return false
}
