# Stage 1: Node — build React web panel
FROM node:20-alpine AS web-builder

WORKDIR /web

COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# Stage 2: Go builder
FROM golang:1.25-bookworm AS builder

# Install Tesseract build dependencies (required for CGO)
RUN apt-get update && apt-get install -y \
    tesseract-ocr \
    libtesseract-dev \
    libleptonica-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copy built React dist so cmd/web can embed it
COPY --from=web-builder /web/dist ./cmd/web/dist

RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/api ./cmd/api
RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/bot ./cmd/bot
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/web ./cmd/web

# Stage 3: runtime
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    tesseract-ocr \
    tesseract-ocr-spa \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /bin/api /bin/api
COPY --from=builder /bin/bot /bin/bot
COPY --from=builder /bin/web /bin/web

# Migrations are embedded at build time via bind-mount or baked in
COPY --from=builder /app/migrations /migrations

# Receipt storage directory
RUN mkdir -p /data/receipts

WORKDIR /
