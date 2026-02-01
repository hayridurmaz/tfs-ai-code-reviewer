# 🤖 AI Pull Request Reviewer for Azure DevOps (Go)

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue.svg)](https://golang.org/)
[![Azure DevOps](https://img.shields.io/badge/Azure%20DevOps-TFS%20/%20Server%20/%20Services-blue.svg)](https://azure.microsoft.com/en-us/products/devops/)

An intelligent, automated code review assistant designed specifically for **Azure DevOps (ADO)** environment. Written in **Go** for maximum performance, parallelism, and resilience.

---

## 🚀 Features

-   **⚡ Multi-Level Parallelism:** Processes multiple repositories, pull requests, and files concurrently using goroutines.
-   **🔍 Incremental Review:** Analyzes only the *actual changes* between iterations, avoiding redundant feedback.
-   **🛡️ Resilience:** Built-in retry mechanism with exponential backoff for transient LLM/API errors.
-   **🧠 LLM Agnostic:** Works with any OpenAI-compatible API (OpenAI, Anthropic via proxy, Ollama, vLLM, etc.).
-   **📝 Senior Engineer Persona:** Provides deep technical insights on security, performance, and architecture.
-   **🗄️ Robust State Management:** Uses SQLite to track reviewed iterations, ensuring zero duplicate comments.
-   **🎯 Advanced Filtering:** Filter by target branches, exclude specific PR IDs, or ignore files via glob patterns.
-   **📜 Lightweight:** Minimal memory footprint and high execution speed.

---

## 🛠️ Tech Stack

-   **Runtime:** Go 1.25+
-   **Database:** SQLite (CGO-free via `modernc.org/sqlite`)
-   **Configuration:** Clean environment-based config via `caarlos0/env`
-   **Diffing:** High-performance diffing via `hexops/gotextdiff`
-   **Context:** Native Go context support for timeouts and graceful shutdown.

---

## ⚙️ Configuration

Create a `.env` file in the root directory or provide environment variables:

```env
# Azure DevOps Configuration
ADO_BASE_URL=https://dev.azure.com/your-org
ADO_PAT=your-personal-access-token
PROJECT_NAME=your-project
REPO_NAMES=repo1,repo2           # Optional: Comma separated repo names
TARGET_BRANCHES=main,develop    # Optional: Filter by target branch
IGNORE_PR_IDS=101,105           # Optional: Exclude specific PRs

# LLM Configuration
LLM_BASE_URL=https://api.openai.com/v1
LLM_API_KEY=your-api-key
LLM_MODEL=gpt-4-turbo
LLM_MAX_RETRIES=3               # Number of retries for transient errors

# Bot Tuning
POLL_INTERVAL_SEC=90            # How often to check for updates
HTTP_TIMEOUT_SEC=30             # Timeout for API requests
MAX_CONCURRENT_PRS=5            # Parallel PR processing limit
MAX_COMMENTS_PER_FILE=3         # Safety limit for AI comments
MIN_CONFIDENCE=0.7              # Min AI confidence to post a comment
MAX_FILE_SIZE_BYTES=50000       # Skip files larger than this
DRY_RUN=false                  # If true, logs review to console only

# Data & Logging
LOG_PATH=data/app.log           # Where to store logs
DB_PATH=data/bot-state.db       # SQLite state database path
IGNORE_PATTERNS=*.md,*.txt      # Global file ignore patterns (glob)
```

---

## 🚦 Getting Started

1.  **Clone and Build:**
    ```bash
    git clone https://github.com/youruser/ai-pr-reviewer.git
    cd ai-pr-reviewer
    go build -o build/reviewer ./cmd/reviewer/main.go
    ```

2.  **Run the bot:**
    ```bash
    ./build/reviewer
    ```

---

## 🐳 Docker

The bot is container-ready with persistent storage for state and logs.

1.  **Build and start:**
    ```bash
    docker-compose up -d --build
    ```

2.  **View logs:**
    ```bash
    docker-compose logs -f
    ```

---

## 🧠 How it Works

1.  **Polling:** The bot monitors repositories for active Pull Requests in parallel.
2.  **Smart Tracking:** It uses a local SQLite database and an in-memory tracker to skip already-reviewed or currently-processing iterations.
3.  **Diff Generation:** Fetches incremental diffs between iterations using native ADO APIs and Git Blobs.
4.  **AI Analysis:** Sends changes to the LLM with a specialized "Senior Staff Engineer" prompt.
5.  **Feedback Integration:** Automatically posts summary summaries and line-level comments back to the PR thread.

---

## 📌 Versioning

This project follows [Semantic Versioning (SemVer)](https://semver.org/).

Versioning and releases are managed through Git tags:
1.  **Tagging:** New versions are created by pushing a tag with the `v` prefix (e.g., `git tag v1.0.0`).
2.  **Automation:** Pushing a tag automatically triggers the GitHub Actions workflow which:
    - Builds the Docker image.
    - Publishes it to **GitHub Container Registry (GHCR)**.
    - Tags it with both the specific version and `latest`.

---

## ⚖️ License

Distributed under the MIT License. See `LICENSE` for more information.

---

Developed with ❤️ for elite code quality.
