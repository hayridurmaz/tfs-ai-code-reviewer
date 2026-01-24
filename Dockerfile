FROM node:20-slim

# Create app directory
WORKDIR /app

# Install build essentials for better-sqlite3 (native bindings)
RUN apt-get update && apt-get install -y \
    python3 \
    make \
    g++ \
    && rm -rf /var/lib/apt/lists/*

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm install --omit=dev

# Copy app source
COPY . .

# Create directory for persistent data
RUN mkdir -p /app/data

# Environment defaults
ENV NODE_ENV=production

# Run the application
CMD ["node", "src/index.js"]
