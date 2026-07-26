package config

import (
	"os"
	"strings"
	"time"
)

const ListenAddr = ":8080"

type Config struct {
	DatabaseURL string
	GitHubToken string
	OpenAIKey   string
	OpenAIBase  string
	OpenAIModel string
	HTTPTimeout time.Duration
}

func FromEnv() Config {
	return Config{
		DatabaseURL: strings.TrimSpace(os.Getenv("DATABASE_URL")),
		GitHubToken: strings.TrimSpace(os.Getenv("GH_DOWNLOADER_GITHUB_TOKEN")),
		OpenAIKey:   strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		OpenAIBase:  strings.TrimRight(strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")), "/"),
		OpenAIModel: strings.TrimSpace(os.Getenv("OPENAI_MODEL")),
		HTTPTimeout: 60 * time.Second,
	}
}
