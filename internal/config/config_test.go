package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Set required env vars
	os.Setenv("ADO_BASE_URL", "https://dev.azure.com/org")
	os.Setenv("ADO_PAT", "dummy-pat")
	os.Setenv("PROJECT_NAME", "dummy-project")
	os.Setenv("LLM_BASE_URL", "https://api.openai.com/v1")
	os.Setenv("LLM_API_KEY", "dummy-key")

	defer func() {
		os.Unsetenv("ADO_BASE_URL")
		os.Unsetenv("ADO_PAT")
		os.Unsetenv("PROJECT_NAME")
		os.Unsetenv("LLM_BASE_URL")
		os.Unsetenv("LLM_API_KEY")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Bot.TimeoutSec != 30 {
		t.Errorf("expected default TimeoutSec 30, got %d", cfg.Bot.TimeoutSec)
	}

	if cfg.Bot.MaxConcurrentPRs != 5 {
		t.Errorf("expected default MaxConcurrentPRs 5, got %d", cfg.Bot.MaxConcurrentPRs)
	}

	if cfg.LLM.MaxRetries != 3 {
		t.Errorf("expected default MaxRetries 3, got %d", cfg.LLM.MaxRetries)
	}
}
