import dotenv from 'dotenv';
dotenv.config();

export default {
    ado: {
        baseUrl: process.env.ADO_BASE_URL,
        pat: process.env.ADO_PAT,
        project: process.env.PROJECT_NAME,
        repos: process.env.REPO_NAMES ? process.env.REPO_NAMES.split(',').map(r => r.trim()) : [],
        targetBranches: process.env.TARGET_BRANCHES ? process.env.TARGET_BRANCHES.split(',').map(b => b.trim()) : [],
        ignorePrIds: process.env.IGNORE_PR_IDS ? process.env.IGNORE_PR_IDS.split(',').map(id => parseInt(id.trim())) : []
    },
    llm: {
        baseUrl: process.env.LLM_BASE_URL,
        apiKey: process.env.LLM_API_KEY,
        model: process.env.LLM_MODEL || 'gpt-4'
    },
    bot: {
        pollIntervalSec: parseInt(process.env.POLL_INTERVAL_SEC || '90'),
        maxCommentsPerFile: parseInt(process.env.MAX_COMMENTS_PER_FILE || '3'),
        minConfidence: parseFloat(process.env.MIN_CONFIDENCE || '0.65'),
        dryRun: process.env.DRY_RUN === 'true',
        maxFileSizeBytes: parseInt(process.env.MAX_FILE_SIZE_BYTES || '20000')
    },
    ignorePatterns: [
        '**/*.min.js',
        '**/dist/**',
        '**/build/**',
        '**/node_modules/**',
        '**/target/**',
        '**/*.lock',
        '**/package-lock.json',
        '**/yarn.lock'
    ]
};