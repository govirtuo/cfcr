package config

import (
	"testing"
)

func Test_isFreqValid(t *testing.T) {
	tests := []struct {
		name string
		f    string
		want bool
	}{
		{name: "hourly", f: "hourly", want: true},
		{name: "daily", f: "daily", want: true},
		{name: "weekly", f: "weekly", want: true},
		{name: "monthly", f: "monthly", want: true},
		{name: "empty", f: "", want: false},
		{name: "random", f: "foobar", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isFreqValid(tt.f); got != tt.want {
				t.Errorf("isFreqValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		fields  Config
		wantErr bool
	}{
		{
			name:    "empty",
			fields:  Config{},
			wantErr: true,
		},
		{
			name: "wrong log level",
			fields: Config{
				LogLevel: "foo",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Config{
				LogLevel: tt.fields.LogLevel,
				Auth:     tt.fields.Auth,
				Checks:   tt.fields.Checks,
				Metrics:  tt.fields.Metrics,
			}
			if err := c.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_isYamlFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{name: ".yaml", filename: "foo.yaml", want: true},
		{name: ".yml", filename: "foo.yml", want: true},
		{name: "wrong extension", filename: "foo.txt", want: false},
		{name: "empty", filename: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isYamlFile(tt.filename); got != tt.want {
				t.Errorf("isYamlFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
