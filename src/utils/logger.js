import winston from 'winston';
import path from 'path';

const LOG_DIR = 'data';

const logger = winston.createLogger({
    level: 'info',
    format: winston.format.combine(
        winston.format.timestamp(),
        winston.format.json()
    ),
    transports: [
        // Dosyaya yaz (detaylı, timestamp'li, json)
        new winston.transports.File({ filename: path.join(LOG_DIR, 'app.log') }),
    ],
});

// Konsola yaz (daha okunabilir, renkli, basit)
if (process.env.NODE_ENV !== 'production') {
    logger.add(new winston.transports.Console({
        format: winston.format.combine(
            winston.format.colorize(),
            winston.format.simple()
        ),
    }));
}

export default logger;
