# Scoring Service Checker (Go)

Lightweight HTTP server that exposes the scoring checks described in `sys.conf`:
HTTP/HTTPS (content and status), SMTP, POP3, FTP (hash/regex), and DNS (A/MX).

## Requirements
- Go 1.22+
- Network access to the target services

## Install
1) Clone this repo.  
2) Download deps (needs network):  
   ```sh
   go mod tidy
   ```
3) Adjust `config.json` (see below) or point to your own file with `CONFIG_PATH`.

## Run
```sh
go run .
```
Server listens on `:8080`. Change by wrapping in a reverse proxy or editing `main.go`.

### Aggregator/reader
Need a small service that polls those checks and returns a single JSON? Run:
```sh
SCORING_BASE_URL=http://127.0.0.1:8080 READER_ADDR=:9090 go run ./cmd/reader
```
It hits the scoring server root to learn the endpoints (falls back to the default list), calls each `/check/*`, and returns an aggregated `{ok, checks[]}` at `/`. `/healthz` reports the reader is up.

## Configuration (`config.json`)
All values can be swapped for your real scoring environment:
- `http` and `https`: `host`, `port`, `path`, `expected_content` (or `regex`), `status`, `allow_insecure_tls` (set true for self-signed certs).
- `smtp`: `host`, `port`, `sender`, `receiver`, `body`.
- `pop3`: `host`, `port`, `use_ssl`, `username`, `password`, `commands[]` (each has `command`, `expected`, `regex`).
- `ftp`: `host`, `port`, `username`, `password`, `files[]` (`name`, optional `sha256`, optional `regex`).
- `dns`: `server`, `port`, `records[]` (`kind` A/MX, `domain`, `answers` list).
- `timeout_seconds`: global per-check timeout.

Key items to change for your deployment:
- Replace placeholder IPs (`10.20.x.2`, etc.) with real hosts.
- Update POP3/SMTP credentials and recipients to valid accounts.
- Adjust FTP usernames/passwords and expected hashes/regex for the files you need to verify.
- Update DNS expected answers to the authoritative records in your event.

You can keep multiple configs and start the server with:
```sh
CONFIG_PATH=./my_config.json go run .
```

## HTTP API
All endpoints return JSON `{ "ok": bool, "detail": string }` and HTTP 200/503.
- `/` – short info and endpoint list
- `/healthz` – always OK if server is up
- `/check/http`
- `/check/https`
- `/check/smtp`
- `/check/pop3`
- `/check/ftp`
- `/check/dns`

## Notes
- DNS MX lookups use UDP against the configured DNS server/port (per `dns` block).
- HTTPS can skip verification when `allow_insecure_tls` is true (useful for self-signed scoring infra).
- If you containerize, remember to pass `CONFIG_PATH` and expose `8080` (or your chosen port).***
