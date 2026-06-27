# syntax=docker/dockerfile:1

# ---------- Stage 1: build the React/Vite SPA ----------
FROM node:18-bookworm-slim AS spa
WORKDIR /app/Client
# git is required: @tailwindcss/typography is pulled from a github: ref
RUN apt-get update && apt-get install -y --no-install-recommends git \
    && rm -rf /var/lib/apt/lists/*
COPY Client/package.json Client/package-lock.json ./
RUN npm ci
COPY Client/ ./
RUN npm run build
# output: /app/Client/dist

# ---------- Stage 2: build the Rust backend ----------
FROM rust:1-bookworm AS backend
WORKDIR /app
COPY Cargo.toml Cargo.lock ./
COPY src ./src
RUN cargo build --release
# output: /app/target/release/scoreboard

# ---------- Stage 3: runtime ----------
FROM debian:bookworm-slim
# Tools the service checkers shell out to (curl/ssh/ftp/dig/nc/python3).
# Without these every check fails, so they are NOT optional.
RUN apt-get update && apt-get install -y --no-install-recommends \
        bash \
        ca-certificates \
        curl \
        dnsutils \
        ftp \
        netcat-openbsd \
        openssh-client \
        python3 \
        sshpass \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=backend /app/target/release/scoreboard /usr/local/bin/scoreboard
COPY --from=spa /app/Client/dist ./public
COPY resources ./resources
# runtime state dirs (gitignored, so not in the build context)
RUN mkdir -p /app/resources/save/autosave

ENV SB_PORT=8000 \
    SB_APP_DIR=/app/public \
    SB_RESOURCE_DIR=/app/resources

EXPOSE 8000
CMD ["scoreboard"]
