import config from './config.js';
import { getRepositories, getActivePullRequests, getIterations, getIterationChanges, getPullRequest } from './ado-client.js';
import { isIterationReviewed, markIterationReviewed } from './state-store.js';
import { reviewPR } from './reviewer.js';
import { publishReview } from './publisher.js';
import logger from './logger.js';

// Tek bir PR'ı işle
async function processPR(repo, pr) {
    try {
        // 3) Iteration'ları kontrol et
        const iterations = await getIterations(repo.id, pr.pullRequestId);
        const latestIteration = iterations[iterations.length - 1];

        if (!latestIteration) return;

        const iterationId = latestIteration.id;

        if (isIterationReviewed(repo.id, pr.pullRequestId, iterationId)) {
            logger.info(`  ⏭️ PR #${pr.pullRequestId} - iteration ${iterationId} zaten incelendi, atlanıyor.`);
            return; // Zaten incelendi
        }

        logger.info(`🔍 PR #${pr.pullRequestId} - iteration ${iterationId} inceleniyor...`);

        // 4) PR detaylarını ve değişiklikleri al
        const [prData, changes] = await Promise.all([
            getPullRequest(repo.id, pr.pullRequestId),
            getIterationChanges(repo.id, pr.pullRequestId, iterationId)
        ]);

        logger.info(`PR Data fetched for #${pr.pullRequestId}`);

        // 5) LLM ile incele
        const reviewResult = await reviewPR(prData, changes);
        logger.info(`Review finished for #${pr.pullRequestId}`);

        // 6) Yorumları yayınla
        await publishReview(repo.id, pr.pullRequestId, iterationId, reviewResult);

        // 7) State'i güncelle
        markIterationReviewed(repo.id, pr.pullRequestId, iterationId, reviewResult);

        logger.info(`✅ PR #${pr.pullRequestId} incelendi ve yorumlar eklendi`);
    } catch (err) {
        logger.error(`❌ PR #${pr.pullRequestId} hatası:`, { message: err.message, stack: err.stack });
    }
}

// Tek bir repoyu işle
async function processRepo(repo) {
    logger.info(`📂 Repo: ${repo.name} (${repo.id})`);

    try {
        // 2) Aktif PR'ları al
        const prs = await getActivePullRequests(repo.id);
        logger.info(`  └─ ${repo.name}: ${prs.length} aktif PR bulundu`);

        // PR'ları paralel işle
        const results = await Promise.allSettled(prs.map(pr => processPR(repo, pr)));

        // Hataları özetle (isteğe bağlı)
        const failed = results.filter(r => r.status === 'rejected');
        if (failed.length > 0) {
            logger.error(`⚠️ ${repo.name} reposunda ${failed.length} PR işlenemedi.`);
        }

    } catch (err) {
        logger.error(`❌ Repo hatası (${repo.name}):`, { message: err.message });
    }
}

async function pollOnce() {
    logger.info(`🔄 Polling başladı...`);

    try {
        // 1) Repoları al
        let repos = await getRepositories();
        if (config.ado.repos.length > 0) {
            repos = repos.filter(r => config.ado.repos.includes(r.name));
        }

        // Repoları paralel işle
        await Promise.allSettled(repos.map(repo => processRepo(repo)));

    } catch (err) {
        logger.error('❌ Polling genel hatası:', { message: err.message });
    }

    logger.info(`✅ Polling tamamlandı. ${config.bot.pollIntervalSec}s sonra tekrar...\n`);
}

async function start() {
    logger.info('🚀 AI PR Reviewer Bot başlatıldı', {
        ado: config.ado.baseUrl,
        project: config.ado.project,
        interval: config.bot.pollIntervalSec
    });

    // İlk çalıştırma
    await pollOnce();

    // Periyodik polling
    setTimeout(start, config.bot.pollIntervalSec * 1000);
}

start();