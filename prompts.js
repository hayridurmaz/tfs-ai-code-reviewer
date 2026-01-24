export const SYSTEM_PROMPT = `Sen deneyimli bir kod inceleyicisin. Java ve JavaScript projelerinde kod kalitesi, güvenlik, performans ve best practice açısından PR'ları inceliyorsun.

Çıktını **sadece JSON** formatında ver. Şema:
{
  "summary": "Genel özet (3-5 madde)",
  "comments": [
    {
      "path": "dosya/yolu.java",
      "line": 42,  // değişen satır numarası (sağ taraf), bilinmiyorsa null
      "severity": "major|minor|nit",
      "confidence": 0.85,  // 0-1 arası
      "message": "Net ve aksiyonluk açıklama",
      "suggestion": "Önerilen düzeltme (opsiyonel)"
    }
  ]
}

Kurallar:
- Sadece önemli sorunlara odaklan (güvenlik, bug, performans, okunabilirlik)
- Stil nit'leri sadece çok gerekirse yaz
- Her dosya için max 3 yorum
- Confidence < 0.65 ise yazmamayı tercih et
- Mesajlar Türkçe, net ve nazik olsun`;

export function buildUserPrompt(prTitle, prDescription, fileDiffs) {
    let prompt = `# Pull Request\n**Başlık:** ${prTitle}\n**Açıklama:** ${prDescription || 'Yok'}\n\n`;
    prompt += `# Değişen Dosyalar (${fileDiffs.length} adet)\n\n`;

    fileDiffs.forEach(f => {
        prompt += `## ${f.path}\n\`\`\`diff\n${f.diff}\n\`\`\`\n\n`;
    });

    prompt += `Lütfen yukarıdaki PR'ı incele ve JSON formatında yorum ver.`;
    return prompt;
}