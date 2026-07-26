package main

import (
	"context"
	"log"
	"net/http"

	"github.com/1996fanrui/gh-downloader/internal/analyzer"
	"github.com/1996fanrui/gh-downloader/internal/app"
	"github.com/1996fanrui/gh-downloader/internal/config"
	gh "github.com/1996fanrui/gh-downloader/internal/github"
	"github.com/1996fanrui/gh-downloader/internal/httpapi"
	"github.com/1996fanrui/gh-downloader/internal/store"
)

func main() {
	cfg := config.FromEnv()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	client := &http.Client{Timeout: cfg.HTTPTimeout}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTPTimeout)
	defer cancel()

	pool, err := store.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	repoStore := store.NewPostgresStore(pool)
	if err := repoStore.Init(ctx); err != nil {
		log.Fatal(err)
	}
	githubClient := gh.NewClient(cfg.GitHubToken, client)
	releaseAnalyzer := analyzer.NewOpenAIAnalyzer(cfg.OpenAIKey, cfg.OpenAIBase, cfg.OpenAIModel, client)
	service := app.NewService(repoStore, githubClient, releaseAnalyzer)
	server := httpapi.NewServer(service, client)

	log.Printf("listening on %s", config.ListenAddr)
	if err := http.ListenAndServe(config.ListenAddr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}
