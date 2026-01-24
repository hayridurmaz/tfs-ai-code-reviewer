# Contributing to AI PR Reviewer

Thank you for your interest in contributing to the AI PR Reviewer! We welcome contributions from the community to make this tool better for everyone.

## 🏁 Getting Started

1.  **Fork the repository** on GitHub.
2.  **Clone your fork** locally:
    ```bash
    git clone https://github.com/your-username/tfs-ai-code-reviewer.git
    cd tfs-ai-code-reviewer
    ```
3.  **Install dependencies**:
    ```bash
    npm install
    ```
4.  **Create a new branch** for your feature or bugfix:
    ```bash
    git checkout -b feature/your-feature-name
    ```

## 🏗️ Project Structure

-   `src/index.js`: Entry point and polling logic.
-   `src/services/`: Core business logic (ADO, LLM, Reviewing).
-   `src/utils/`: Shared utilities (Logger, SQLite store).
-   `src/config/`: Configuration management.
-   `data/`: Local persistent storage (gitignored).

## 🛠️ Development Guidelines

-   **ES Modules**: Use modern `import/export` syntax.
-   **Logging**: Use the built-in `logger` from `src/utils/logger.js` instead of `console.log`.
-   **State**: If you add new tracking features, update the schema in `src/utils/state-store.js`.
-   **Dry Run**: Always test your changes with `DRY_RUN=true` to avoid spamming pull requests during development.

## 🧪 Testing

Before submitting a Pull Request, please ensure:
-   The code follows the project's structure.
-   Logging is properly implemented.
-   Native dependencies (like SQLite) work across environments.

## 📥 Submitting a PR

1.  Commit your changes with clear, descriptive messages.
2.  Push to your fork.
3.  Open a Pull Request against the `main` branch of the original repository.
4.  Provide a clear description of the changes and link any related issues.

## 🐞 Reporting Issues

Use the **Issue Templates** to report bugs or suggest features. Be as descriptive as possible, including environment details and steps to reproduce.

---

Thank you for making AI PR Reviewer better! 🚀
