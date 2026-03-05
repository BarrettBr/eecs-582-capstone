FROM python:3.11-slim AS base
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    WORKDIR=/ml

RUN useradd -m -u 1000 appuser

# Install system dependencies (may not be needed here)
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# --- Stage 2: Development ---
FROM base as development

# Copy over all files in the ml app
COPY --chown=appuser:appuser ./ml/app/ .
COPY --chown=appuser:appuser ./docker/ml/entrypoint.sh /entrypoint.sh

RUN chmod u+x /entrypoint.sh &&\
	pip install --no-cache-dir -r requirements.txt

USER appuser
ENTRYPOINT ["/entrypoint.sh"]
