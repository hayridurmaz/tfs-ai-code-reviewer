import config from './config.js';
import { getRepositories, getActivePullRequests, getIterations, getIterationChanges, getPullRequest } from './ado-client.js';
import { isIterationReviewed, markIterationReviewed } from './state-store.js';
import { reviewPR } from './reviewer.js';
import { publishReview } from './publisher.js';

// Tek bir PR'ı işle
async function processPR(repo, pr) {
    try {
        // 3) Iteration'ları kontrol et
        const iterations = await getIterations(repo.id, pr.pullRequestId);
        const latestIteration = iterations[iterations.length - 1];

        if (!latestIteration) return;

        const iterationId = latestIteration.id;

        if (isIterationReviewed(repo.id, pr.pullRequestId, iterationId)) {
            return; // Zaten incelendi
        }

        console.log(`  🔍 PR #${pr.pullRequestId} - iteration ${iterationId} inceleniyor...`);

        // 4) PR detaylarını ve değişiklikleri al
        const [prData, changes] = await Promise.all([
            getPullRequest(repo.id, pr.pullRequestId),
            getIterationChanges(repo.id, pr.pullRequestId, iterationId)
        ]);

        console.log(`PR Data fetched for #${pr.pullRequestId}`);

        // 5) LLM ile incele
        const reviewResult = await reviewPR(prData, changes);
        console.log(`Review finished for #${pr.pullRequestId}`);

        // 6) Yorumları yayınla
        await publishReview(repo.id, pr.pullRequestId, iterationId, reviewResult);

        // 7) State'i güncelle
        markIterationReviewed(repo.id, pr.pullRequestId, iterationId);

        console.log(`  ✅ PR #${pr.pullRequestId} incelendi ve yorumlar eklendi`);
    } catch (err) {
        console.error(`❌ PR #${pr.pullRequestId} hatası:`, err.message);
    }
}

// Tek bir repoyu işle
async function processRepo(repo) {
    console.log(`📂 Repo: ${repo.name} (${repo.id})`);

    try {
        // 2) Aktif PR'ları al
        const prs = await getActivePullRequests(repo.id);
        console.log(`  └─ ${repo.name}: ${prs.length} aktif PR bulundu`);

        // PR'ları paralel işle
        const results = await Promise.allSettled(prs.map(pr => processPR(repo, pr)));

        // Hataları özetle (isteğe bağlı)
        const failed = results.filter(r => r.status === 'rejected');
        if (failed.length > 0) {
            console.error(`  ⚠️ ${repo.name} reposunda ${failed.length} PR işlenemedi.`);
        }

    } catch (err) {
        console.error(`❌ Repo hatası (${repo.name}):`, err.message);
    }
}

async function pollOnce() {
    console.log(`🔄 Polling başladı... (${new Date().toISOString()})`);

    try {
        // 1) Repoları al
        let repos = await getRepositories();
        if (config.ado.repos.length > 0) {
            repos = repos.filter(r => config.ado.repos.includes(r.name));
        }

        // Repoları paralel işle
        await Promise.allSettled(repos.map(repo => processRepo(repo)));

    } catch (err) {
        console.error('❌ Polling genel hatası:', err.message);
    }

    console.log(`✅ Polling tamamlandı. ${config.bot.pollIntervalSec}s sonra tekrar...\n`);
}

async function start() {
    console.log('🚀 AI PR Reviewer Bot başlatıldı');
    console.log(`   ADO: ${config.ado.baseUrl}`);
    console.log(`   Proje: ${config.ado.project}`);
    console.log(`   Interval: ${config.bot.pollIntervalSec}s\n`);

    // İlk çalıştırma
    await pollOnce();

    // Periyodik polling
    setTimeout(start, config.bot.pollIntervalSec * 1000);
}

start();