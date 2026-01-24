import axios from 'axios';
import config from '../config/index.js';

const client = axios.create({
    baseURL: config.ado.baseUrl,
    headers: {
        'Authorization': `Basic ${Buffer.from(':' + config.ado.pat).toString('base64')}`,
        'Content-Type': 'application/json'
    }
});

// Tüm repoları listele
export async function getRepositories() {
    const url = `/${config.ado.project}/_apis/git/repositories?api-version=6.0`;
    const res = await client.get(url);
    return res.data.value;
}

// Aktif PR'ları listele
export async function getActivePullRequests(repoId) {
    const url = `/${config.ado.project}/_apis/git/repositories/${repoId}/pullrequests?searchCriteria.status=active&api-version=6.0`;
    const res = await client.get(url);
    return res.data.value;
}

// PR iteration'larını al
export async function getIterations(repoId, prId) {
    const url = `/${config.ado.project}/_apis/git/repositories/${repoId}/pullrequests/${prId}/iterations?api-version=6.0`;
    const res = await client.get(url);
    return res.data.value;
}

// Bir iteration'daki değişiklikleri al (diff dahil)
// compareToIterationId verilirse incremental diff (iki iteration arası fark) döner
export async function getIterationChanges(repoId, prId, iterationId, compareToIterationId = null) {
    let url = `/${config.ado.project}/_apis/git/repositories/${repoId}/pullrequests/${prId}/iterations/${iterationId}/changes?api-version=6.0`;

    // Eğer karşılaştırma yapılacak iteration ID varsa ekle
    if (compareToIterationId) {
        url += `&$compareTo=${compareToIterationId}`;
    }

    const res = await client.get(url);
    const changes = res.data.changeEntries || [];

    // URL eksikse Blob URL'i ile tamamla (ADO bazen dönmeyebiliyor)
    changes.forEach(change => {
        const baseUrl = config.ado.baseUrl.endsWith('/') ? config.ado.baseUrl.slice(0, -1) : config.ado.baseUrl;

        if (change.item && change.item.objectId) {
            // $format=octetStream ekleyerek RAW içeriği almayı garanti ediyoruz (yoksa JSON metadata döner)
            if (!change.item.url) {
                change.item.url = `${baseUrl}/${config.ado.project}/_apis/git/repositories/${repoId}/blobs/${change.item.objectId}?api-version=6.0&$format=octetStream`;
            }
        }

        // Eğer dosya değişmişse (edit), eski versiyonu da çekmek için originalUrl oluştur
        if (change.item && change.item.originalObjectId) {
            change.item.originalUrl = `${baseUrl}/${config.ado.project}/_apis/git/repositories/${repoId}/blobs/${change.item.originalObjectId}?api-version=6.0&$format=octetStream`;
        }
    });

    return changes;
}

// PR detaylarını al (başlık, açıklama)
export async function getPullRequest(repoId, prId) {
    const url = `/${config.ado.project}/_apis/git/repositories/${repoId}/pullrequests/${prId}?api-version=6.0`;
    const res = await client.get(url);
    return res.data;
}

// Thread (yorum) oluştur
export async function createThread(repoId, prId, thread) {
    const url = `/${config.ado.project}/_apis/git/repositories/${repoId}/pullrequests/${prId}/threads?api-version=6.0`;
    const res = await client.post(url, thread);
    return res.data;
}

// Dosya içeriğini url üzerinden al (text olarak)
export async function getFileContent(downloadUrl) {
    // ADO bazen absolut URL bazen relative verebilir, genelde tam URL döner ama auth header eklememiz lazım.
    // client.get(url) kullanırsak baseURL eklenebilir, o yüzden tam URL ise direct axios ya da config ile oynamak lazım.
    // Ancak client baseURL tanımlı olduğu için, eğer downloadUrl tam URL ise client.get bizi üzebilir.
    // Çoğu durumda item.url tam path döner: https://dev.azure.com/...

    // Basit çözüm: Eğer URL http ile başlıyorsa direkt axios (headerları kopyalayarak), yoksa client.

    if (downloadUrl.startsWith('http')) {
        const res = await axios.get(downloadUrl, {
            headers: {
                'Authorization': `Basic ${Buffer.from(':' + config.ado.pat).toString('base64')}`
            },
            responseType: 'text'
        });
        return res.data;
    } else {
        const res = await client.get(downloadUrl, { responseType: 'text' });
        return res.data;
    }
}
