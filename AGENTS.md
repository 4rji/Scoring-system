# Repository Guidelines

## Project Structure & Module Organization

This repository contains two related scoring systems. The root Go service is in `main.go`, with the polling aggregator in `cmd/reader/`, configuration examples in `config.json` and `sys.conf`, and helper logic in `service_checks.py`. The `metrosco/` directory contains the Metro CCDC scoreboard: Rust backend code in `metrosco/src/`, React/TypeScript client code in `metrosco/Client/src/`, built web assets in `metrosco/public/`, and checker scripts plus sample game data in `metrosco/resources/`.

## Build, Test, and Development Commands

- `go run .` starts the root scoring checker on `:8080`.
- `SCORING_BASE_URL=http://127.0.0.1:8080 READER_ADDR=:9090 go run ./cmd/reader` starts the Go aggregate reader.
- `go test ./...` runs all Go package tests and compile checks.
- `cd metrosco && cargo run -r` runs the Rust scoreboard, normally on `http://localhost:8000`.
- `cd metrosco && cargo test` runs Rust tests.
- `cd metrosco/Client && npm run dev` starts the Vite development server.
- `cd metrosco/Client && npm run build` runs TypeScript checking and builds the React app.
- `cd metrosco && just buildspa` builds the client and replaces `metrosco/public/` with the generated SPA.

## Coding Style & Naming Conventions

Use `gofmt` for Go and keep tests in `*_test.go` files beside the package they cover. Use `cargo fmt` for Rust; keep modules snake_case and public types in PascalCase. The React app uses TypeScript, function components, PascalCase component files under `Pages/` and `Components/`, and camelCase hooks/utilities. Follow the existing Tailwind utility style for UI changes.

## Testing Guidelines

There are few tests today, so new behavior should include focused coverage when practical. Add Go tests beside the relevant package, Rust unit tests inline or integration tests under `metrosco/tests/`, and frontend tests as `*.test.tsx` if a test runner is introduced. Always run the matching build/test command before submitting changes.

## Commit & Pull Request Guidelines

Recent history is terse and inconsistent (`12`, `naal2`, merge commits), so prefer clearer commits going forward: short imperative subject lines such as `Add POP3 timeout handling` or `Fix scoreboard inject parsing`. Pull requests should describe the change, list validation commands run, link related issues, and include screenshots for visible React UI changes. Note any required environment variables such as `CONFIG_PATH`, `SB_RESOURCE_DIR`, `SB_PORT`, or `SB_ADMIN_PASSWORD`.

## Security & Configuration Tips

Do not commit real team credentials, mail passwords, or event IPs. Keep deploy-specific values in local config files or environment variables, and use sample data in `metrosco/resources/` only when it is safe to share.
