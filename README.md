# 🤖 AI Pull Request Reviewer for Azure DevOps

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Node.js Version](https://img.shields.io/badge/node-%3E%3D18.0.0-brightgreen.svg)](https://nodejs.org/)
[![Azure DevOps](https://img.shields.io/badge/Azure%20DevOps-TFS%20/%20Server%20/%20Services-blue.svg)](https://azure.microsoft.com/en-us/products/devops/)

An intelligent, automated code review assistant designed specifically for **Azure DevOps (ADO)** environment. It monitors your pull requests, analyzes changes using Large Language Models (LLMs), and provides high-quality, actionable feedback directly as PR comments.

---

## 🚀 Features

-   **⚡ Parallel Processing:** Efficiently monitors multiple repositories and pull requests simultaneously.
-   **🔍 Incremental Review:** Smart enough to analyze only the *actual changes* between iterations, avoiding redundant feedback.
-   **🧠 LLM Agnostic:** Works with any OpenAI-compatible API (OpenAI, Anthropic via proxy, Ollama, vLLM, etc.).
-   **📝 Senior Engineer Persona:** Provides deep technical insights on security, performance, and architecture rather than trivial style issues.
-   **🛡️ State Management:** Uses SQLite to keep track of reviewed iterations and posted comments, ensuring zero duplicate noise.
-   **🎯 Advanced Filtering:** Filter by target branches or exclude specific PR IDs via configuration.
-   **📜 Detailed Logging:** Professional file and console logging powered by Winston.

---

## 🛠️ Tech Stack

-   **Runtime:** Node.js (ES Modules)
-   **API Client:** Axios
-   **Database:** SQLite (Better-SQLite3)
-   **Logging:** Winston
-   **Diffing:** `diff` package for patch generation

---

## ⚙️ Configuration

Create a `.env` file in the root directory based on `.env.example`:

```env
# Azure DevOps Configuration
ADO_BASE_URL=https://dev.azure.com/your-org
ADO_PAT=your-personal-access-token
PROJECT_NAME=your-project
REPO_NAMES=repo1,repo2           # Comma separated (Optional: reviews all if empty)
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

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/youruser/ai-pr-reviewer.git
    cd ai-pr-reviewer
    ```

2.  **Install dependencies:**
    ```bash
    npm install
    ```

3.  **Configure environment:**
    ```bash
    cp .env.example .env
    # Edit .env with your credentials
    ```

4.  **Run the bot:**
    ```bash
    npm start
    ```

---

## 🐳 Docker

The easiest way to run the reviewer in production is using Docker Compose.

1.  **Build and start:**
    ```bash
    docker-compose up -d --build
    ```

2.  **View logs:**
    ```bash
    docker-compose logs -f
    ```

Your database (`bot-state.db`) and logs (`app.log`) will be persisted in the project directory even if the container is removed.

---

## 🧠 How it Works

1.  **Polling:** The bot periodically checks for active Pull Requests in configured repositories.
2.  **Iteration Detection:** It identifies the latest iteration of a PR and checks the local SQLite database to see if it has been reviewed.
3.  **Diff Generation:**
    *   If it's a new PR, it fetches all changes.
    *   If it's an update, it fetches an **incremental diff** between the current and the previous iteration.
4.  **Analysis:** The unified patch is sent to the LLM with a specialized "Senior Staff Engineer" system prompt.
5.  **Feedback:** The AI generates a summary and specific line-level comments with:
    *   `severity` (major/minor/nit)
    *   `message` (technical explanation)
    *   `suggestion` (actual code snippets)
6.  **Persistence:** Comments are posted to ADO and the state is updated in the database.

---

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

---

## ⚖️ License

Distributed under the MIT License. See `LICENSE` for more information.

---

Developed with ❤️ for better code quality.
