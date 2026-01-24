import { reviewCode } from './llm-client.js';
import { SYSTEM_PROMPT, buildUserPrompt } from './prompts.js';
import config from './config.js';
import minimatch from 'minimatch';
import { getFileContent } from './ado-client.js';

function shouldIgnoreFile(path) {
    return config.ignorePatterns.some(pattern => minimatch(path, pattern));
}

async function prepareFileContext(changeEntry) {
    if (!changeEntry.item || !changeEntry.item.path) return null;
    if (changeEntry.changeType === 'delete') return null;
    if (shouldIgnoreFile(changeEntry.item.path)) return null;

    // Uzantı kontrolü (örneğin sadece kod dosyaları)
    // Şimdilik ignorePatterns yeterli

    // Dosya içeriğini çek
    try {
        if (!changeEntry.item.url) return null;

        // ADO change entry içinde bazen size bilgisi olmayabilir ama genelde header'dan vs anlaşılır.
        // Biz burada indirdikten sonra veya indirmeden önce kontrol edebiliriz.
        // ADO API get changes bazen size dönmez.

        const content = await getFileContent(changeEntry.item.url);

        if (Buffer.byteLength(content, 'utf8') > config.bot.maxFileSizeBytes) {
            console.log(`⚠️ Dosya çok büyük, atlanıyor: ${changeEntry.item.path}`);
            return null;
        }

        return {
            path: changeEntry.item.path,
            diff: content, // Şimdilik diff yerine full content gönderiyoruz
            changeType: changeEntry.changeType
        };

    } catch (err) {
        console.error(`❌ Dosya indirilemedi: ${changeEntry.item.path}`, err.message);
        return null;
    }
}

export async function reviewPR(prData, changes) {
    const promises = changes.map(prepareFileContext);
    const fileDiffs = (await Promise.all(promises)).filter(f => f !== null);

    if (fileDiffs.length === 0) {
        return { summary: 'İncelenecek uygun dosya bulunamadı (ignore rules veya size limit).', comments: [] };
    }

    const userPrompt = buildUserPrompt(prData.title, prData.description, fileDiffs);

    console.log(`🧠 LLM'e gönderiliyor... (${fileDiffs.length} dosya)`);
    const result = await reviewCode(SYSTEM_PROMPT, userPrompt);

    // Güven filtresi
    result.comments = result.comments.filter(c => c.confidence >= config.bot.minConfidence);

    // Dosya başına max yorum
    const grouped = {};
    result.comments.forEach(c => {
        if (!grouped[c.path]) grouped[c.path] = [];
        grouped[c.path].push(c);
    });

    const filtered = [];
    Object.values(grouped).forEach(comments => {
        filtered.push(...comments.slice(0, config.bot.maxCommentsPerFile));
    });

    result.comments = filtered;
    return result;
}