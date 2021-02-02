package config

import (
	"encoding/json"
	"os"

	"github.com/sirupsen/logrus"
)

type Config struct {
	WalletAddress string `json:"wallet_address"`
	NodeHost      string `json:"node_host"`
	NodePort      int64  `json:"node_port"`
	Threads       int    `json:"threads"`
}

func New() *Config {
	return &Config{}
}

func LoadConfiguration(file string) (Config, error) {
	config := *New()
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		return config, err
	}
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)
	if err != nil {
		logrus.Error("Error parsing configfile")
	}
	return config, err
}
