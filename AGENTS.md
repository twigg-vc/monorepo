## Guidelines

Your name is Cláudio and my name is Supreme Leader.
ALWAYS refer to you and me with those names.

- Break complex tasks into atomic stacked commits — never one huge commit.
- Write tests. Write code that is easy to review.

## What This Is

Twigg: a version control service built around small, incremental changes.

| Package | Purpose |
|---|---|
| `twigg` | Core library (shared by all packages) |
| `twigg-web` | Main web server (repos, reviews, CI/CD UI, payments) |
| `twigg-track` | CI/CD server (runs jobs, stores logs, webhooks back) |
| `twigg-runner` | Binary that runs a job payload's commands |
| `twigg-vscode` | VS Code extension |
| `data` | Storage libraries (blob storage, SQLite helpers) |
| `squeue` | SQLite-backed queue |
| `base` | Shared utilities |

## Commands

```bash
make test-all               # all Go tests, monorepo-wide
cd twigg-web && make test   # twigg-web tests
cd twigg-track && make test # twigg-track tests
```

## Go Style

Non-standard style:

- Each package has an `aa_public.go` with **all** public exports and nothing else. Keep it small; implementations >10 lines go in separate files.
- Behavioral structs (services, handlers, stores): public struct + private `impl` field + forwarding methods + `NewXXX` constructor. Don't return interfaces from constructors (legacy style — don't imitate).
- Plain data structs: no impl split, but still a `NewXXX` constructor.
- Define interfaces at the consumer; accept interfaces, return structs.
- In twigg-web, handler contexts carry a DB transaction: `HandleFuncR`/`HandleFuncW` open a read/write transaction and pass it as the handler's `context.Context` (the mux commits/closes it). Service and DB methods thread it through as a plain `context.Context` param named `r` (read) or `w` (write) — never open your own transaction inside a handler. Details and rationale in `STYLE.md`.

## TypeScript / JavaScript Style

No ternaries. Use explicit `if/else` with both branches. Prefer `var x = undefined` + `if/else` over default-then-overwrite. All *web components* must use `twigg-web/webcomponents/twigg.css` design tokens.

## Architecture

**twigg-web** — single self-contained binary:
- `handlers/` — HTTP handlers, reached through layered mux wrappers (`wrappers/`) that inject request data.
- `services/` — business logic, consumed by handlers.
- `webdb/` — DB implementation. New code defines the minimal DB interface it needs; don't import the DB directly. Table setup and simple CRUD belong on WebDb, not in services — legacy services are being migrated one commit at a time; see `twigg-web/SERVICE_TO_WEBDB.md`.
- Frontend: Lit components (`/twigg-web/webcomponents`) (TypeScript, esbuild); Embedded into the binary.
- Docs: Docusaurus (`twigg-web/docusaurus`). Embedded into the binary.

**twigg-track** — separate server. Receives job events from twigg-web via webhook, runs them in containers/VMs (using `twigg-runner`), stores logs, streams logs, and webhooks status back.

## Version Control

@TWIGG.md

## Commit messages

Use commit messages in the "<verb> <thing>" format.
Examples:
- `Define user entity`
- `Implement user methods in the database`
- `Add test to user handler`
COUNTER examples - DONT use this:
- `feat: enable sign up`
- `webdb: add CreateBug`