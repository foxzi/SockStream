# Конфигурация SockStream

## Источники конфигурации

SockStream поддерживает три уровня конфигурации (в порядке приоритета):

1. **CLI-флаги** — наивысший приоритет
2. **Переменные окружения** — средний приоритет
3. **Файл конфигурации** — базовый уровень

## Файл конфигурации

Поддерживаются форматы YAML и TOML. Расширения: `.yaml`, `.yml`, `.toml`.

### Полный пример YAML

```yaml
listen: 0.0.0.0:8080
host_name: example.com
target: https://target.example.com

proxy:
  # Вариант 1: Один прокси (legacy)
  type: socks5
  address: 127.0.0.1:1080
  auth:
    username: user
    password: pass

  # Вариант 2: Пул прокси с URL форматом (рекомендуется)
  urls:
    - socks5://user:pass@proxy1:1080
    - socks5://proxy2:1080
    - http://user:pass@httpproxy:8080
    - https://secureproxy:443
  rotation: round-robin  # или "random"

  timeouts:
    connect_seconds: 10
    idle_seconds: 30

access:
  allow:
    - 192.168.0.0/16
    - 10.0.0.0/8
  block:
    - 192.168.1.100/32

cors:
  allowed_origins:
    - "*"
  allowed_headers:
    - "*"
  allow_credentials: false
  allow_methods:
    - GET
    - POST
    - PUT
    - DELETE
    - OPTIONS
  max_age_seconds: 600

headers:
  rewrite_host: true
  rewrite_origin: true
  rewrite_referer: true
  add:
    X-Forwarded-Proto: https
    X-Custom-Header: value

logging:
  level: info

tls:
  cert_file: /path/to/cert.pem
  key_file: /path/to/key.pem
  acme:
    enabled: false
    domain: example.com
    email: admin@example.com
    cache_dir: acme-cache
    http01_port: "80"
```

## Переменные окружения

Префикс: `SOCKSTREAM_`

| Переменная | Описание |
|------------|----------|
| `SOCKSTREAM_LISTEN` | Адрес для прослушивания |
| `SOCKSTREAM_HOST_NAME` | Переопределение Host заголовка |
| `SOCKSTREAM_TARGET` | Целевой URL (обязательно) |
| `SOCKSTREAM_PROXY_TYPE` | Тип прокси: `direct`, `http`, `https`, `socks5` |
| `SOCKSTREAM_PROXY_ADDRESS` | Адрес прокси-сервера |
| `SOCKSTREAM_PROXY_USERNAME` | Имя пользователя прокси |
| `SOCKSTREAM_PROXY_PASSWORD` | Пароль прокси |
| `SOCKSTREAM_PROXY_URLS` | Список прокси URL (через запятую) |
| `SOCKSTREAM_PROXY_ROTATION` | Стратегия ротации: `round-robin`, `random` |
| `SOCKSTREAM_ALLOW_IPS` | Разрешённые CIDR (через запятую) |
| `SOCKSTREAM_BLOCK_IPS` | Заблокированные CIDR (через запятую) |
| `SOCKSTREAM_CORS_ORIGINS` | Разрешённые источники CORS |
| `SOCKSTREAM_ADD_HEADERS` | Доп. заголовки (`key=value,key2=value2`) |
| `SOCKSTREAM_TLS_CERT_FILE` | Путь к сертификату |
| `SOCKSTREAM_TLS_KEY_FILE` | Путь к ключу |
| `SOCKSTREAM_ACME_DOMAIN` | Домен для ACME (включает ACME) |
| `SOCKSTREAM_ACME_EMAIL` | Email для ACME |
| `SOCKSTREAM_ACME_CACHE_DIR` | Директория кэша ACME |

## CLI-флаги

```
-config string      Путь к файлу конфигурации
-listen string      Адрес для прослушивания (по умолчанию: 0.0.0.0:8080)
-target string      Целевой URL (обязательно)
-host-name string   Переопределение Host заголовка
-proxy-type string  Тип прокси (direct/http/https/socks5)
-proxy-addr string  Адрес прокси
-proxy-user string  Имя пользователя прокси
-proxy-pass string  Пароль прокси
-allow string       Разрешённые CIDR
-cors string        CORS origins
-headers string     Доп. заголовки
-tls-cert string    Путь к TLS сертификату
-tls-key string     Путь к TLS ключу
-acme-domain string Домен для ACME
-acme-email string  Email для ACME
-no-rewrite-host    Отключить перезапись Host
```

## Типы прокси

| Тип | Описание |
|-----|----------|
| `direct` или пусто | Прямое подключение |
| `http` | HTTP прокси |
| `https` | HTTPS прокси |
| `socks5` | SOCKS5 прокси |

## Пул прокси (Proxy Pool)

SockStream поддерживает пул прокси с автоматической ротацией.

### URL формат

```
scheme://[user:password@]host:port
```

Примеры:
- `socks5://proxy.example.com:1080`
- `socks5://user:pass@proxy.example.com:1080`
- `http://admin:secret@httpproxy.local:8080`
- `https://secureproxy.com:443`

### Стратегии ротации

| Стратегия | Описание |
|-----------|----------|
| `round-robin` | Последовательный перебор (по умолчанию) |
| `random` | Случайный выбор |

### Пример использования

```yaml
proxy:
  urls:
    - socks5://user:pass@proxy1.example.com:1080
    - socks5://proxy2.example.com:1080
    - http://httpproxy.example.com:8080
  rotation: round-robin
```

Через переменные окружения:

```bash
export SOCKSTREAM_PROXY_URLS="socks5://proxy1:1080,http://proxy2:8080"
export SOCKSTREAM_PROXY_ROTATION="random"
```

## Контроль доступа

- Блок-лист проверяется первым (deny имеет приоритет)
- Если allow-лист пуст — разрешены все IP
- Поддерживаются IPv4 и IPv6 CIDR
- IP клиента извлекается из `X-Forwarded-For` или `RemoteAddr`

## Заголовки (Headers)

Настройка перезаписи и добавления HTTP-заголовков при проксировании.

### Перезапись заголовков

| Параметр | Описание |
|----------|----------|
| `rewrite_host` | Заменяет заголовок `Host` на хост из `target` |
| `rewrite_origin` | Заменяет заголовок `Origin` на URL из `target` |
| `rewrite_referer` | Заменяет заголовок `Referer` на URL из `target` |

**Зачем это нужно:** Многие серверы проверяют эти заголовки для защиты от несанкционированного доступа. Если `Host` не совпадает с ожидаемым доменом — сервер может отклонить запрос. `Origin` и `Referer` проверяются для защиты от CSRF-атак.

### Пример работы

```
# rewrite_host: true
Запрос клиента:    Host: localhost:8080
После перезаписи:  Host: target.example.com

# rewrite_origin: true
Запрос клиента:    Origin: http://localhost:8080
После перезаписи:  Origin: https://target.example.com

# rewrite_referer: true
Запрос клиента:    Referer: http://localhost:8080/page
После перезаписи:  Referer: https://target.example.com
```

### Добавление заголовков

Секция `add` позволяет добавить произвольные заголовки к каждому запросу:

```yaml
headers:
  rewrite_host: true
  rewrite_origin: true
  rewrite_referer: true
  add:
    X-Forwarded-Proto: https
    X-Custom-Header: my-value
    Authorization: Bearer token123
```

## TLS

### Ручные сертификаты

```yaml
tls:
  cert_file: /path/to/cert.pem
  key_file: /path/to/key.pem
```

### ACME (Let's Encrypt)

```yaml
tls:
  acme:
    enabled: true
    domain: example.com
    email: admin@example.com
    cache_dir: acme-cache
    http01_port: "80"
```

Требуется открытый порт 80 для HTTP-01 challenge.
