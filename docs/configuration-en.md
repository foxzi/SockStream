# SockStream Configuration

## Configuration Sources

SockStream supports three configuration levels (in priority order):

1. **CLI flags** — highest priority
2. **Environment variables** — medium priority
3. **Configuration file** — base level

## Configuration File

Supported formats: YAML and TOML. Extensions: `.yaml`, `.yml`, `.toml`.

### Full YAML Example

```yaml
listen: 0.0.0.0:8080
host_name: example.com
target: https://target.example.com

proxy:
  # Option 1: Single proxy (legacy)
  type: socks5
  address: 127.0.0.1:1080
  auth:
    username: user
    password: pass

  # Option 2: Proxy pool with URL format (recommended)
  urls:
    - socks5://user:pass@proxy1:1080
    - socks5://proxy2:1080
    - http://user:pass@httpproxy:8080
    - https://secureproxy:443
  rotation: round-robin  # or "random"

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
    - "X-Forwarded-Proto: https"
    - "X-Custom-Header: value"
  delete:
    - X-Forwarded-For
    - X-Real-IP

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

## Environment Variables

Prefix: `SOCKSTREAM_`

| Variable | Description |
|----------|-------------|
| `SOCKSTREAM_LISTEN` | Listen address |
| `SOCKSTREAM_HOST_NAME` | Override Host header |
| `SOCKSTREAM_TARGET` | Target URL (required) |
| `SOCKSTREAM_PROXY_TYPE` | Proxy type: `direct`, `http`, `https`, `socks5` |
| `SOCKSTREAM_PROXY_ADDRESS` | Proxy server address |
| `SOCKSTREAM_PROXY_USERNAME` | Proxy username |
| `SOCKSTREAM_PROXY_PASSWORD` | Proxy password |
| `SOCKSTREAM_PROXY_URLS` | List of proxy URLs (comma-separated) |
| `SOCKSTREAM_PROXY_ROTATION` | Rotation strategy: `round-robin`, `random` |
| `SOCKSTREAM_ALLOW_IPS` | Allowed CIDRs (comma-separated) |
| `SOCKSTREAM_BLOCK_IPS` | Blocked CIDRs (comma-separated) |
| `SOCKSTREAM_CORS_ORIGINS` | Allowed CORS origins |
| `SOCKSTREAM_ADD_HEADERS` | Additional headers (`key=value,key2=value2`) |
| `SOCKSTREAM_TLS_CERT_FILE` | Path to certificate |
| `SOCKSTREAM_TLS_KEY_FILE` | Path to key |
| `SOCKSTREAM_ACME_DOMAIN` | ACME domain (enables ACME) |
| `SOCKSTREAM_ACME_EMAIL` | ACME email |
| `SOCKSTREAM_ACME_CACHE_DIR` | ACME cache directory |

## CLI Flags

```
-config string      Path to configuration file
-listen string      Listen address (default: 0.0.0.0:8080)
-target string      Target URL (required)
-host-name string   Override Host header
-proxy-type string  Proxy type (direct/http/https/socks5)
-proxy-addr string  Proxy address
-proxy-user string  Proxy username
-proxy-pass string  Proxy password
-allow string       Allowed CIDRs
-cors string        CORS origins
-headers string     Additional headers
-tls-cert string    Path to TLS certificate
-tls-key string     Path to TLS key
-acme-domain string ACME domain
-acme-email string  ACME email
-no-rewrite-host    Disable Host rewriting
```

## Proxy Types

| Type | Description |
|------|-------------|
| `direct` or empty | Direct connection |
| `http` | HTTP proxy |
| `https` | HTTPS proxy |
| `socks5` | SOCKS5 proxy |

## Proxy Pool

SockStream supports a proxy pool with automatic rotation.

### URL Format

```
scheme://[user:password@]host:port
```

Examples:
- `socks5://proxy.example.com:1080`
- `socks5://user:pass@proxy.example.com:1080`
- `http://admin:secret@httpproxy.local:8080`
- `https://secureproxy.com:443`

### Rotation Strategies

| Strategy | Description |
|----------|-------------|
| `round-robin` | Sequential rotation (default) |
| `random` | Random selection |

### Usage Example

```yaml
proxy:
  urls:
    - socks5://user:pass@proxy1.example.com:1080
    - socks5://proxy2.example.com:1080
    - http://httpproxy.example.com:8080
  rotation: round-robin
```

Via environment variables:

```bash
export SOCKSTREAM_PROXY_URLS="socks5://proxy1:1080,http://proxy2:8080"
export SOCKSTREAM_PROXY_ROTATION="random"
```

## Access Control

- Block list is checked first (deny takes precedence)
- If allow list is empty — all IPs are permitted
- IPv4 and IPv6 CIDRs are supported
- Client IP is extracted from `X-Forwarded-For` or `RemoteAddr`

## Headers

Configuration for rewriting and adding HTTP headers during proxying.

### Header Rewriting

| Parameter | Description |
|-----------|-------------|
| `rewrite_host` | Replaces `Host` header with host from `target` |
| `rewrite_origin` | Replaces `Origin` header with URL from `target` |
| `rewrite_referer` | Replaces `Referer` header with URL from `target` |

**Why is this needed:** Many servers check these headers for security. If `Host` doesn't match the expected domain, the server may reject the request or redirect. `Origin` and `Referer` are checked for CSRF protection.

### Example

```
# rewrite_host: true
Client request:    Host: localhost:8080
After rewrite:     Host: target.example.com

# rewrite_origin: true
Client request:    Origin: http://localhost:8080
After rewrite:     Origin: https://target.example.com

# rewrite_referer: true
Client request:    Referer: http://localhost:8080/page
After rewrite:     Referer: https://target.example.com
```

### Adding Headers

The `add` section allows adding custom headers to every request:

```yaml
headers:
  add:
    - "X-Forwarded-Proto: https"
    - "X-Custom-Header: my-value"
    - "Authorization: Bearer token123"
```

### Deleting Headers

The `delete` section allows removing headers from the request before forwarding to the target server:

```yaml
headers:
  delete:
    - X-Forwarded-For
    - X-Real-IP
    - X-Custom-Header
```

**Why is this needed:** Deleting headers is useful when:
- You need to hide the client's real IP from the target server (`X-Forwarded-For`, `X-Real-IP`)
- You need to remove headers added by intermediate proxies
- You need to clean up service headers before proxying

**Processing order:** `delete` is executed first, then `rewrite_*`, then `add`.

## TLS

### Manual Certificates

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

Port 80 must be open for HTTP-01 challenge.
