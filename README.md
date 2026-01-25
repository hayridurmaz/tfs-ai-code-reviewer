# 🤖 AI Pull Request Reviewer for Azure DevOps (Go)

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue.svg)](https://golang.org/)
[![Azure DevOps](https://img.shields.io/badge/Azure%20DevOps-TFS%20/%20Server%20/%20Services-blue.svg)](https://azure.microsoft.com/en-us/products/devops/)

An intelligent, automated code review assistant designed specifically for **Azure DevOps (ADO)** environment. Now rewritten in **Go** for maximum performance and easier deployment.

---

## 🚀 Features

-   **⚡ Parallel Processing:** Efficiently monitors multiple repositories and pull requests simultaneously.
-   **🔍 Incremental Review:** Smart enough to analyze only the *actual changes* between iterations, avoiding redundant feedback.
-   **🧠 LLM Agnostic:** Works with any OpenAI-compatible API (OpenAI, Anthropic via proxy, Ollama, vLLM, etc.).
-   **📝 Senior Engineer Persona:** Provides deep technical insights on security, performance, and architecture rather than trivial style issues.
-   **🛡️ State Management:** Uses SQLite to keep track of reviewed iterations and posted comments, ensuring zero duplicate noise.
-   **🎯 Advanced Filtering:** Filter by target branches or exclude specific PR IDs via configuration.
-   **📜 Performance:** Lightweight Go binary with minimal memory footprint.

---

## 🛠️ Tech Stack

-   **Runtime:** Go 1.21+
-   **Database:** SQLite (CGO-free via `modernc.org/sqlite`)
-   **Configuration:** Clean environment-based config via `caarlos0/env`
-   **Diffing:** High-performance diffing via `hexops/gotextdiff`

---

## ⚙️ Configuration

Create a `.env` file in the root directory:

```env
# Azure DevOps Configuration
ADO_BASE_URL=https://dev.azure.com/your-org
ADO_PAT=your-personal-access-token
PROJECT_NAME=your-project
REPO_NAMES=repo1,repo2           # Comma separated (Optional)
TARGET_BRANCHES=main,develop    # Filter by target branch (Optional)
IGNORE_PR_IDS=101,105           # Exclude specific PRs (Optional)

# LLM Configuration
LLM_BASE_URL=https://api.openai.com/v1
LLM_API_KEY=your-api-key
LLM_MODEL=gpt-4-turbo           # Or your local/private model

# Bot Tuning
POLL_INTERVAL_SEC=90
MAX_COMMENTS_PER_FILE=3
MIN_CONFIDENCE=0.7
MAX_FILE_SIZE_BYTES=50000
DRY_RUN=false                  # If true, logs review to console without posting to ADO
```

---

## 🚦 Getting Started

1.  **Clone and Build:**
    ```bash
    git clone https://github.com/youruser/ai-pr-reviewer.git
    cd ai-pr-reviewer
    go build -o reviewer ./cmd/reviewer/main.go
    ```

2.  **Run the bot:**
    ```bash
    ./reviewer
    ```

---

## 🐳 Docker

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

1.  **Polling:** The bot periodically checks for active Pull Requests in configured repositories.
2.  **Iteration Detection:** It identifies the latest iteration of a PR and checks the local SQLite database.
3.  **Diff Generation:** Fetches incremental diffs between iterations using native ADO APIs and high-performance Go diffing.
4.  **Analysis:** The unified patch is sent to the LLM with a specialized "Senior Staff Engineer" system prompt.
5.  **Feedback:** The AI generates a summary and specific line-level comments.
6.  **Persistence:** Comments are posted to ADO and the state is updated in SQLite.

---

## 🤝 Contributing

Any contributions you make are **greatly appreciated**.

1.  Fork the Project
2.  Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3.  Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4.  Push to the Branch (`git push origin feature/AmazingFeature`)
5.  Open a Pull Request

---

## ⚖️ License

Distributed under the MIT License. See `LICENSE` for more information.

---

Developed with ❤️ for better code quality.
