import axios from 'axios';
import config from './config.js';

const client = axios.create({
    baseURL: config.llm.baseUrl,
    headers: {
        'Authorization': `Bearer ${config.llm.apiKey}`,
        'Content-Type': 'application/json'
    }
});

export async function reviewCode(systemPrompt, userPrompt) {
    const res = await client.post('/chat/completions', {
        model: config.llm.model,
        messages: [
            { role: 'system', content: systemPrompt },
            { role: 'user', content: userPrompt }
        ],
        temperature: 0.3,
        response_format: { type: 'json_object' } // OpenAI JSON mode (destekliyorsa)
    });

    const content = res.data.choices[0].message.content;
    return JSON.parse(content);
}