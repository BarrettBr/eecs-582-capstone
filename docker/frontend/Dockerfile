FROM node:20-alpine AS base

WORKDIR /app

# ----- Dependencies stage -----
FROM base AS dependencies

# Copy package files
COPY frontend/package.json frontend/package-lock.json* ./

# Install dependencies deterministically from lockfile.
RUN npm ci --no-audit --no-fund

FROM base AS development

# Copy dependencies from dependencies stage
COPY --from=dependencies /app/node_modules ./node_modules

# Copy source code
COPY frontend/ .

# Expose Vite dev server port
EXPOSE 5173

# Run Vite dev server
CMD ["npm", "run", "dev", "--", "--host"]
