package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	UDP struct {
		Port        int  `mapstructure:"port"`
		LogRequests bool `mapstructure:"log_requests"`
	} `mapstructure:"udp"`
	API struct {
		Port        int  `mapstructure:"port"`
		LogRequests bool `mapstructure:"log_requests"`
	} `mapstructure:"api"`
	Redis struct {
		Enabled  bool   `mapstructure:"enabled"`
		Address  string `mapstructure:"address"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
		Channel  string `mapstructure:"channel"`
	} `mapstructure:"redis"`
	Agones struct {
		Enabled             bool   `mapstructure:"enabled"`
		Namespace           string `mapstructure:"namespace"`
		AllocatorHost       string `mapstructure:"allocator_host"`
		AllocatorClientCert string `mapstructure:"allocator_client_cert"`
		AllocatorClientKey  string `mapstructure:"allocator_client_key"`
		AllocatorCACert     string `mapstructure:"allocator_ca_cert"`
	} `mapstructure:"agones"`
	Routes []struct {
		FQDN   string `mapstructure:"fqdn"`
		Type   string `mapstructure:"type"`
		Target string `mapstructure:"target"`
	} `mapstructure:"routes"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	viper.SetDefault("udp.port", 443)
	viper.SetDefault("udp.log_requests", false)
	viper.SetDefault("api.port", 8080)
	viper.SetDefault("api.log_requests", false)
	viper.SetDefault("redis.enabled", false)
	viper.SetDefault("redis.channel", "porter_routes")
	viper.SetDefault("agones.enabled", false)
	viper.SetDefault("agones.namespace", "default")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
