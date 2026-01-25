package config

import (
	"strings"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
)

type Config struct {
	ADO struct {
		BaseURL        string   `env:"ADO_BASE_URL,required"`
		PAT            string   `env:"ADO_PAT,required"`
		ProjectName    string   `env:"PROJECT_NAME,required"`
		Repos          []string `env:"REPO_NAMES" envSeparator:","`
		TargetBranches []string `env:"TARGET_BRANCHES" envSeparator:","`
		IgnorePRIDs    []int    `env:"IGNORE_PR_IDS" envSeparator:","`
	}
	LLM struct {
		BaseURL    string `env:"LLM_BASE_URL,required"`
		APIKey     string `env:"LLM_API_KEY,required"`
		Model      string `env:"LLM_MODEL" envDefault:"gpt-4-turbo"`
		MaxRetries int    `env:"LLM_MAX_RETRIES" envDefault:"3"`
	}
	Bot struct {
		PollIntervalSec    int     `env:"POLL_INTERVAL_SEC" envDefault:"90"`
		MaxCommentsPerFile int     `env:"MAX_COMMENTS_PER_FILE" envDefault:"3"`
		MinConfidence      float64 `env:"MIN_CONFIDENCE" envDefault:"0.7"`
		MaxFileSizeBytes   int64   `env:"MAX_FILE_SIZE_BYTES" envDefault:"50000"`
		DryRun             bool    `env:"DRY_RUN" envDefault:"false"`
		LogPath            string  `env:"LOG_PATH" envDefault:"data/app.log"`
		DBPath             string  `env:"DB_PATH" envDefault:"data/bot-state.db"`
		MaxConcurrentPRs   int     `env:"MAX_CONCURRENT_PRS" envDefault:"5"`
	}
	IgnorePatterns []string `env:"IGNORE_PATTERNS" envSeparator:"," envDefault:"*.md,*.txt,package-lock.json,yarn.lock"`
}

func Load() (*Config, error) {
	// Try loading from .env file but don't fail if it doesn't exist
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	// Clean up branch names if they have refs/heads/ prefix
	for i, branch := range cfg.ADO.TargetBranches {
		cfg.ADO.TargetBranches[i] = strings.TrimPrefix(branch, "refs/heads/")
	}

	return cfg, nil
}

func (c *Config) ShouldIgnoreFile(path string) bool {
	// This will be implemented using gobwas/glob in the reviewer package or here
	// For now, simple check or placeholder
	return false
}
