package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// RepoConfig is one manifest entry.
type RepoConfig struct {
	Repo   string `mapstructure:"repo"`
	Graph  string `mapstructure:"graph"`
	Commit string `mapstructure:"commit"`
	Branch string `mapstructure:"branch"`

	OpenAPI []string `mapstructure:"openapi"`

	Proto struct {
		Roots []string `mapstructure:"roots"`
		Files []string `mapstructure:"files"`
	} `mapstructure:"proto"`

	Go struct {
		Modules         []string `mapstructure:"modules"`
		PrivatePrefixes []string `mapstructure:"private_prefixes"`
	} `mapstructure:"go"`
}

// Config is the parsed manifest.
type Config struct {
	Repos []RepoConfig `mapstructure:"repos"`
}

// Org returns the org segment of "org/name".
func (r RepoConfig) Org() string {
	parts := strings.SplitN(r.Repo, "/", 2)
	return parts[0]
}

// Name returns the name segment of "org/name".
func (r RepoConfig) Name() string {
	parts := strings.SplitN(r.Repo, "/", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func (r RepoConfig) validate() (RepoConfig, error) {
	if !strings.Contains(r.Repo, "/") {
		return r, fmt.Errorf("repo %q must be in org/name form", r.Repo)
	}
	if r.Graph == "" {
		return r, fmt.Errorf("repo %q missing graph path", r.Repo)
	}
	return r, nil
}

// Load reads and validates a viper manifest at path.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	for i, r := range cfg.Repos {
		if _, err := r.validate(); err != nil {
			return nil, err
		}
		_ = i
	}
	return &cfg, nil
}
