import crypto from 'crypto';
import { createThread } from './ado-client.js';
import { isCommentPosted, markCommentPosted } from './state-store.js';
import config from './config.js';

function makeFingerprint(repoId, prId, iterationId, path, message) {
    const str = `${repoId}:${prId}:${iterationId}:${path}:${message.substring(0, 50)}`;
    return crypto.createHash('md5').update(str).digest('hex');
}

export async function publishReview(repoId, prId, iterationId, reviewResult) {
    if (config.bot.dryRun) {
        console.log(`\n🚧 DRY RUN MODE: Review Result for PR #${prId} Iteration #${iterationId}`);
        console.log('--------------------------------------------------');
        console.log(`Summary: ${reviewResult.summary}`);
        console.log('Comments:');
        reviewResult.comments.forEach(c => {
            console.log(`[${c.severity}] ${c.path}:${c.line || 'FILE'} - ${c.message}`);
        });
        console.log('--------------------------------------------------\n');
        return;
    }

    const threads = [];

    // 1) Özet yorumu (genel, satırsız)
    let summaryText = reviewResult.summary;
    if (Array.isArray(summaryText)) {
        summaryText = summaryText.join('\n\n');
    }

    const summaryFp = makeFingerprint(repoId, prId, iterationId, '__summary__', summaryText);
    if (!isCommentPosted(summaryFp)) {
        threads.push({
            comments: [{
                parentCommentId: 0,
                content: `🤖 **AI Code Review (Iteration ${iterationId})**\n\n${summaryText}`,
                commentType: 1
            }],
            status: 1 // active
        });
        markCommentPosted(summaryFp);
    }

    // 2) Inline yorumlar (dosya + satır bazlı)
    for (const comment of reviewResult.comments) {
        // ADO path'lerin başında '/' olmasını bekler
        const cleanPath = comment.path.startsWith('/') ? comment.path : '/' + comment.path;

        const fp = makeFingerprint(repoId, prId, iterationId, cleanPath, comment.message);
        if (isCommentPosted(fp)) continue;

        const severityEmoji = { major: '🔴', minor: '🟡', nit: '⚪' }[comment.severity] || '🔵';
        const content = `${severityEmoji} **${comment.severity.toUpperCase()}** (güven: ${Math.round(comment.confidence * 100)}%)\n\n${comment.message}${comment.suggestion ? `\n\n💡 Öneri:\n\`\`\`\n${comment.suggestion}\n\`\`\`` : ''}`;

        const thread = {
            comments: [{
                parentCommentId: 0,
                content,
                commentType: 1
            }],
            status: 1
        };

        // Eğer satır bilgisi varsa threadContext ekle (inline yorum)
        if (comment.line) {
            thread.threadContext = {
                filePath: cleanPath,
                rightFileStart: { line: comment.line, offset: 1 },
                rightFileEnd: { line: comment.line, offset: 1 }
            };
        } else {
            // Satır yoksa dosya seviyesinde thread (V1 fallback)
            thread.threadContext = {
                filePath: cleanPath
            };
        }

        threads.push(thread);
        markCommentPosted(fp);
    }

    // Tüm thread'leri yaz
    for (const thread of threads) {
        try {
            await createThread(repoId, prId, thread);
            console.log(`✅ Yorum yazıldı: ${thread.threadContext?.filePath || 'özet'}`);
        } catch (err) {
            console.error(`❌ Yorum yazılamadı:`, err.response?.data || err.message);
        }
    }
}