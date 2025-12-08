# SockStream

Легковесный HTTP/HTTPS reverse proxy с поддержкой SOCKS5 и HTTP(S) прокси, переписывания заголовков и CORS.

*Lightweight HTTP/HTTPS reverse proxy with SOCKS5 and HTTP(S) proxy support, header rewriting and CORS.*

## Как это работает / How it works

```
                                    ┌─────────────────────────────────────────┐
                                    │            Proxy Pool                   │
                                    │  ┌───────┐ ┌───────┐ ┌───────┐         │
                                    │  │Proxy 1│ │Proxy 2│ │Proxy 3│  ...    │
                                    │  │  OK   │ │  OK   │ │TIMEOUT│         │
                                    │  └───┬───┘ └───┬───┘ └───────┘         │
                                    │      │         │                        │
                                    │      └────┬────┘                        │
                                    │           │ Health Check (5 min)        │
┌────────┐     ┌─────────────┐      │           ▼                             │
│ Client │────▶│ SockStream  │──────┼──▶ Round-Robin / Random                 │
└────────┘     │             │      │           │                             │
               │ - CORS      │      │           │ Retry on timeout            │
               │ - ACL       │      │           ▼                             │
               │ - Headers   │      │    ┌─────────────┐                      │
               │ - TLS/ACME  │      │    │ Target Host │                      │
               └─────────────┘      │    └─────────────┘                      │
                                    └─────────────────────────────────────────┘
```

**Основные возможности:**

| Функция | Описание |
|---------|----------|
| Proxy Pool | Пул прокси с автоматической ротацией (round-robin / random) |
| Health Check | Проверка доступности прокси каждые 5 минут |
| Auto Retry | При таймауте автоматический переход на следующий прокси |
| Header Rewrite | Перезапись Host, Origin, Referer для совместимости с целевым сервером |
| Access Control | Фильтрация по IP (allow/block списки, IPv4/IPv6) |
| TLS | Поддержка сертификатов и автоматический ACME (Let's Encrypt) |

**Key features:**

| Feature | Description |
|---------|-------------|
| Proxy Pool | Pool of proxies with automatic rotation (round-robin / random) |
| Health Check | Proxy availability check every 5 minutes |
| Auto Retry | Automatic failover to next proxy on timeout |
| Header Rewrite | Rewrite Host, Origin, Referer for target server compatibility |
| Access Control | IP filtering (allow/block lists, IPv4/IPv6) |
| TLS | Certificate support and automatic ACME (Let's Encrypt) |

## Установка / Installation

### Docker

```bash
docker pull ghcr.io/foxzi/sockstream:latest
docker run -p 8080:8080 ghcr.io/foxzi/sockstream -target https://example.com
```

### Debian/Ubuntu

```bash
# Скачайте актуальную версию со страницы релизов
# Download latest version from releases page
wget https://github.com/foxzi/SockStream/releases/latest/download/sockstream_X.X.X_amd64.deb
sudo dpkg -i sockstream_*.deb
sudo systemctl enable --now sockstream
```

### RHEL/CentOS/Fedora

```bash
# Скачайте актуальную версию со страницы релизов
# Download latest version from releases page
wget https://github.com/foxzi/SockStream/releases/latest/download/sockstream-X.X.X-1.x86_64.rpm
sudo rpm -i sockstream-*.rpm
sudo systemctl enable --now sockstream
```

### Binary

Скачайте бинарник для вашей платформы со [страницы релизов](https://github.com/foxzi/SockStream/releases).

*Download binary for your platform from [releases page](https://github.com/foxzi/SockStream/releases).*

### From source

```bash
go build ./cmd/sockstream
./sockstream -config config.example.yaml
```

## Быстрый старт / Quick start

```bash
./sockstream -target https://example.com
```

Статус / Health check: `GET /healthz` возвращает `200 ok`.

## Конфигурация

Поддерживаются YAML/TOML файл, переменные окружения (`SOCKSTREAM_*`) и CLI-флаги.

Пример `config.example.yaml`:

```yaml
listen: 0.0.0.0:8080
host_name: example.com
target: https://target.com
proxy:
  type: socks5
  address: 127.0.0.1:1080
  # auth:
  #   username: user
  #   password: pass
cors:
  allowed_origins:
    - "*"
access:
  allow:
    - 0.0.0.0/0
headers:
  rewrite_host: true
  rewrite_origin: true
  rewrite_referer: true
  add:
    X-Forwarded-Proto: https
```

Ключевые переменные окружения:

- `SOCKSTREAM_LISTEN`, `SOCKSTREAM_TARGET`, `SOCKSTREAM_HOST_NAME`
- `SOCKSTREAM_PROXY_TYPE`, `SOCKSTREAM_PROXY_ADDRESS`, `SOCKSTREAM_PROXY_USERNAME`, `SOCKSTREAM_PROXY_PASSWORD`
- `SOCKSTREAM_ALLOW_IPS`, `SOCKSTREAM_BLOCK_IPS` (список CIDR через запятую)
- `SOCKSTREAM_CORS_ORIGINS`, `SOCKSTREAM_ADD_HEADERS` (`key=value,key2=value2`)
- `SOCKSTREAM_TLS_CERT_FILE`, `SOCKSTREAM_TLS_KEY_FILE`, `SOCKSTREAM_ACME_DOMAIN`, `SOCKSTREAM_ACME_EMAIL`, `SOCKSTREAM_ACME_CACHE_DIR`

## Сборка Docker-образа

```bash
docker build -t sockstream .
docker run -p 8080:8080 sockstream -target https://example.org
```

## Основные функции

- HTTP/HTTPS сервер с CORS, логированием и ACL по IP
- Проксирование HTTP/WebSocket к целевому хосту с перепиской Host/Origin/Referer и добавлением заголовков
- Исходящие соединения через SOCKS5 или HTTP(S) прокси
- TLS: ручные сертификаты или ACME/Let's Encrypt (HTTP-01)
- Один бинарник без внешних зависимостей времени выполнения

## Документация / Documentation

- [Конфигурация (RU)](docs/configuration.md)
- [Configuration (EN)](docs/configuration-en.md)
