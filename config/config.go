package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Bot     BotConfig     `toml:"bot"`
	Plugins PluginConfig  `toml:"plugins"`
}

type BotConfig struct {
	Prefix      string `toml:"prefix"`
	SessionPath string `toml:"session_path"`
	DbPath      string `toml:"db_path"`
}

type PluginConfig struct {
	Directory string `toml:"directory"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
