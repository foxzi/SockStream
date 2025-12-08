# SockStream

Легковесный HTTP/HTTPS reverse proxy с поддержкой SOCKS5 и HTTP(S) прокси, переписывания заголовков и CORS.

## Быстрый старт

```bash
go build ./cmd/sockstream
./sockstream -config config.example.yaml
```

Статус: `GET /healthz` возвращает `200 ok`.

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
- TLS: ручные сертификаты или ACME/Let’s Encrypt (HTTP-01)
- Один бинарник без внешних зависимостей времени выполнения
