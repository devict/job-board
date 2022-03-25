package main

import (
	"fmt"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	URL         string `envconfig:"APP_URL" required:"true" default:"http://localhost:8080"`
	Port        string `envconfig:"PORT" required:"true" default:":8080"`
	Env         string `envconfig:"APP_ENV" required:"true" default:"debug"`
	AppSecret   string `envconfig:"APP_SECRET" required:"true"`
	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`
	Email       EmailConfig
	Twitter     TwitterConfig
	SlackHook   string `envconfig:"SLACK_HOOK"`
}

type EmailConfig struct {
	SMTPHost     string `envconfig:"SMTP_HOST" required:"true"`
	FromEmail    string `envconfig:"FROM_EMAIL" required:"true"`
	SMTPUsername string `envconfig:"SMTP_USERNAME" required:"true"`
	SMTPPassword string `envconfig:"SMTP_PASSWORD" required:"true"`
}

type TwitterConfig struct {
	AccessToken       string `envconfig:"TW_ACCESS_TOKEN"`
	AccessTokenSecret string `envconfig:"TW_ACCESS_TOKEN_SECRET"`
	APIKey            string `envconfig:"TW_API_KEY"`
	APISecretKey      string `envconfig:"TW_API_SECRET_KEY"`
}

func LoadConfig() (Config, error) {
	var config Config

	if err := envconfig.Process("", &config); err != nil {
		return config, err
	}

	if !strings.Contains(config.DatabaseURL, "sslmode=disable") {
		config.DatabaseURL = fmt.Sprintf("%s?sslmode=disable", config.DatabaseURL)
	}

	if !strings.Contains(config.Port, ":") {
		config.Port = ":" + config.Port
	}

	return config, nil
}
