import config from './config.js';
import { getRepositories, getActivePullRequests, getIterations, getIterationChanges, getPullRequest } from './ado-client.js';
import { isIterationReviewed, markIterationReviewed } from './state-store.js';
import { reviewPR } from './reviewer.js';
import { publishReview } from './publisher.js';

async function pollOnce() {
    console.log(`🔄 Polling başladı... (${new Date().toISOString()})`);

    try {
        // 1) Repoları al
        let repos = await getRepositories();
        if (config.ado.repos.length > 0) {
            repos = repos.filter(r => config.ado.repos.includes(r.name));
        }

        for (const repo of repos) {
            console.log(`📂 Repo: ${repo.name} (${repo.id})`);

            // 2) Aktif PR'ları al
            const prs = await getActivePullRequests(repo.id);
            console.log(`  └─ ${prs.length} aktif PR bulundu`);

            for (const pr of prs) {
                // 3) Iteration'ları kontrol et
                const iterations = await getIterations(repo.id, pr.pullRequestId);
                const latestIteration = iterations[iterations.length - 1];

                if (!latestIteration) continue;

                const iterationId = latestIteration.id;

                if (isIterationReviewed(repo.id, pr.pullRequestId, iterationId)) {
                    continue; // Zaten incelendi
                }

                console.log(`  🔍 PR #${pr.pullRequestId} - iteration ${iterationId} inceleniyor...`);

                // 4) PR detaylarını ve değişiklikleri al
                const prData = await getPullRequest(repo.id, pr.pullRequestId);
                const changes = await getIterationChanges(repo.id, pr.pullRequestId, iterationId);

                // 5) LLM ile incele
                const reviewResult = await reviewPR(prData, changes);

                // 6) Yorumları yayınla
                await publishReview(repo.id, pr.pullRequestId, iterationId, reviewResult);

                // 7) State'i güncelle
                markIterationReviewed(repo.id, pr.pullRequestId, iterationId);

                console.log(`  ✅ PR #${pr.pullRequestId} incelendi ve yorumlar eklendi`);
            }
        }
    } catch (err) {
        console.error('❌ Polling hatası:', err.message);
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
    setInterval(pollOnce, config.bot.pollIntervalSec * 1000);
}

start();