import { reviewCode } from './llm.service.js';
import { SYSTEM_PROMPT, buildUserPrompt } from './prompts.js';
import config from '../config/index.js';
import { minimatch } from 'minimatch';
import { getFileContent } from './ado.service.js';
import logger from '../utils/logger.js';
import { createPatch } from 'diff';

function shouldIgnoreFile(path) {
    return config.ignorePatterns.some(pattern => minimatch(path, pattern));
}

async function prepareFileContext(changeEntry) {
    if (!changeEntry.item || !changeEntry.item.path) return null;
    if (changeEntry.changeType === 'delete') return null;
    if (shouldIgnoreFile(changeEntry.item.path)) return null;

    // Dosya içeriğini çek
    try {
        if (!changeEntry.item.url) return null;

        const contentPromise = getFileContent(changeEntry.item.url);
        let originalContentPromise = Promise.resolve('');

        // Eğer edit ise ve originalUrl varsa eski içeriği de çek
        if (changeEntry.changeType === 'edit' && changeEntry.item.originalUrl) {
            originalContentPromise = getFileContent(changeEntry.item.originalUrl).catch(err => {
                logger.warn(`Eski içerik indirilemedi (${changeEntry.item.path}): ${err.message}`);
                return '';
            });
        }

        const [content, originalContent] = await Promise.all([contentPromise, originalContentPromise]);

        if (Buffer.byteLength(content, 'utf8') > config.bot.maxFileSizeBytes) {
            logger.warn(`⚠️ Dosya çok büyük, atlanıyor: ${changeEntry.item.path}`);
            return null;
        }

        // Diff oluştur
        const diffPatch = createPatch(changeEntry.item.path, originalContent, content);

        return {
            path: changeEntry.item.path,
            diff: diffPatch,
            changeType: changeEntry.changeType
        };

    } catch (err) {
        logger.error(`❌ Dosya indirilemedi: ${changeEntry.item.path}`, { message: err.message });
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

    logger.info(`🧠 LLM'e gönderiliyor... (${fileDiffs.length} dosya)`);
    const result = await reviewCode(SYSTEM_PROMPT, userPrompt);

    // Güven filtresi
    if (!result || !Array.isArray(result.comments)) {
        logger.warn('⚠️ LLM geçersiz yanıt döndürdü, comments array eksik:', result);
        return { summary: 'LLM yanıtı işlenemedi.', comments: [] };
    }

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
