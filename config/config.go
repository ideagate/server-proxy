package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Redis Redis `yaml:"redis"`
}

type Redis struct {
	Addrs    []string `yaml:"addrs"`
	Password string   `yaml:"password"`
	DB       int      `yaml:"db"`
}

func NewConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
