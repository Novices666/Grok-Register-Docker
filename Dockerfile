# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26.5

FROM golang:${GO_VERSION}-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags "-s -w -X main.version=0.1.0" -o /out/grok ./cmd/grok \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags "-s -w" -o /out/grok-web ./cmd/web

FROM mcr.microsoft.com/playwright/python:v1.55.0-noble

ENV DEBIAN_FRONTEND=noninteractive \
    GROK_HOME=/data/grok \
    GROK_PYTHON=/opt/cloakbrowser-venv/bin/python \
    GROK_TURNSTILE_SCRIPT=/usr/local/share/grok-reg/turnstile_mint.py \
    GROK_TURNSTILE_POOL_SCRIPT=/usr/local/share/grok-reg/turnstile_pool.py \
    CLOAKBROWSER_SUPPRESS_FONT_WARNING=1

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        netcat-openbsd \
        procps \
        python3.12-venv \
        tini \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt/Grok-Register
COPY scripts/requirements-turnstile.txt ./scripts/requirements-turnstile.txt
COPY scripts/turnstile_mint.py ./scripts/turnstile_mint.py
COPY scripts/turnstile_pool.py ./scripts/turnstile_pool.py

RUN python -m venv /opt/cloakbrowser-venv \
    && /opt/cloakbrowser-venv/bin/pip install --no-cache-dir -U pip \
    && /opt/cloakbrowser-venv/bin/pip install --no-cache-dir -r ./scripts/requirements-turnstile.txt \
    && /opt/cloakbrowser-venv/bin/python -m cloakbrowser install

COPY --from=builder /out/grok /usr/local/bin/grok
COPY --from=builder /out/grok-web /usr/local/bin/grok-web
COPY scripts/turnstile_mint.py /usr/local/share/grok-reg/turnstile_mint.py
COPY scripts/turnstile_pool.py /usr/local/share/grok-reg/turnstile_pool.py
COPY docker/entrypoint.sh /usr/local/bin/grok-docker-entrypoint

RUN chmod +x /usr/local/bin/grok /usr/local/bin/grok-web /usr/local/bin/grok-docker-entrypoint /usr/local/share/grok-reg/turnstile_mint.py /usr/local/share/grok-reg/turnstile_pool.py \
    && mkdir -p /data/grok

VOLUME ["/data/grok"]
ENTRYPOINT ["tini", "--", "grok-docker-entrypoint"]
CMD ["run"]
