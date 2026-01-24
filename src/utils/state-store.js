import Database from 'better-sqlite3';
import path from 'path';

const DB_PATH = path.join('data', 'bot-state.db');
const db = new Database(DB_PATH);

db.exec(`
  CREATE TABLE IF NOT EXISTS reviewed_iterations (
    repo_id TEXT,
    pr_id INTEGER,
    iteration_id INTEGER,
    reviewed_at TEXT,
    review_result TEXT,
    PRIMARY KEY (repo_id, pr_id, iteration_id)
  )
`);

db.exec(`
  CREATE TABLE IF NOT EXISTS posted_comments (
    fingerprint TEXT PRIMARY KEY,
    posted_at TEXT
  )
`);

export function isIterationReviewed(repoId, prId, iterationId) {
    const row = db.prepare('SELECT 1 FROM reviewed_iterations WHERE repo_id=? AND pr_id=? AND iteration_id=?')
        .get(repoId, prId, iterationId);
    return !!row;
}

export function markIterationReviewed(repoId, prId, iterationId, reviewResult) {
    const resultJson = reviewResult ? JSON.stringify(reviewResult) : null;
    db.prepare('INSERT OR IGNORE INTO reviewed_iterations (repo_id, pr_id, iteration_id, reviewed_at, review_result) VALUES (?, ?, ?, ?, ?)')
        .run(repoId, prId, iterationId, new Date().toISOString(), resultJson);
}

export function isCommentPosted(fingerprint) {
    const row = db.prepare('SELECT 1 FROM posted_comments WHERE fingerprint=?').get(fingerprint);
    return !!row;
}

export function markCommentPosted(fingerprint) {
    db.prepare('INSERT OR IGNORE INTO posted_comments VALUES (?, ?)')
        .run(fingerprint, new Date().toISOString());
}
