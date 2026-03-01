package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/ado"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/config"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/llm"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/orchestrator"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/publisher"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/reviewer"
	"github.com/hayridurmaz/tfs-ai-code-reviewer/internal/state"
	"github.com/sirupsen/logrus"
)

func main() {
	// 1) Load config
	cfg, err := config.Load()
	if err != nil {
		logrus.Fatalf("Failed to load config: %v", err)
	}

	// 2) Setup workspace and logging
	initLogger(cfg)

	logrus.Infof("🚀 AI PR Reviewer Bot (Go) starting...")
	logrus.Infof("ADO: %s, Project: %s, Interval: %ds", cfg.ADO.BaseURL, cfg.ADO.ProjectName, cfg.Bot.PollIntervalSec)

	// 3) Init services
	store, err := state.NewStore(cfg.Bot.DBPath)
	if err != nil {
		logrus.Fatalf("Failed to init state store: %v", err)
	}
	defer store.Close()

	adoClient := ado.NewClient(cfg.ADO.BaseURL, cfg.ADO.ProjectName, cfg.ADO.PAT, cfg.Bot.TimeoutSec)
	llmClient := llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.MaxRetries)
	rev := reviewer.NewReviewer(cfg, adoClient, llmClient)
	pub := publisher.NewPublisher(adoClient)

	// 4) Init Orchestrator
	orc := orchestrator.NewOrchestrator(cfg, store, adoClient, rev, pub)

	// 5) Start Polling loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logrus.Info("Gracefully shutting down...")
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			orc.PollOnce(ctx)
			logrus.Debugf("✅ Polling completed. Waiting %ds...", cfg.Bot.PollIntervalSec)

			select {
			case <-time.After(time.Duration(cfg.Bot.PollIntervalSec) * time.Second):
			case <-ctx.Done():
				return
			}
		}
	}
}

func initLogger(cfg *config.Config) {
	// Ensure directories exist
	if err := os.MkdirAll(filepath.Dir(cfg.Bot.LogPath), 0755); err != nil {
		logrus.Fatalf("Failed to create log directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Bot.DBPath), 0755); err != nil {
		logrus.Fatalf("Failed to create DB directory: %v", err)
	}

	// Setup logrus
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	logFile, err := os.OpenFile(cfg.Bot.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logrus.Fatalf("Failed to open log file: %v", err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	logrus.SetOutput(mw)
}
