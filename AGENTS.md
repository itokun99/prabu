# Prabu — Homelab AI Bootstrap CLI

**Stack:** Go 1.26, standard library only. Prabu installs and configures ZeroClaw and OMO Native on a user-scoped Linux/systemd host.

## Architecture

- ZeroClaw is the Telegram-facing assistant and owns long polling, authorization, chat sessions, and forum-topic routing.
- Telegram group replies require a mention or reply to the bot; direct messages are always processed.
- ZeroClaw delegates executable work through its shell tool using OMO Native's `omo -p` print mode.
- Never enable or call ZeroClaw's `opencode_cli` tool.
- Telegram does not use a webhook, public URL, or inbound port.
- Bot credentials must be stored through ZeroClaw's secret-aware config interface; never write a Telegram token to TOML, logs, or command-line arguments.

## Structure

```text
prabu/
├── main.go          # Signal-aware CLI entry point
├── cli.go           # Command dispatch and flag parsing
├── setup.go         # ZeroClaw/OMO installation and service setup
├── setup_input.go   # Setup flags, input validation, and confirmation
├── setup_config.go  # Config patch, agent instructions, systemd PATH override
├── install.go       # Installer download and subprocess execution
├── manifest.go      # Managed install state
├── doctor.go        # Runtime readiness checks
├── lifecycle.go     # Update and uninstall
├── config.go        # Legacy env loader retained for compatibility tests; unused by CLI
├── bot/             # Legacy bridge package retained for compatibility/tests
├── mcp/             # Legacy MCP package retained for compatibility/tests
└── README.md        # User-facing setup and lifecycle guide
```

The root `prabu` executable does not import or start the legacy `bot/` or `mcp/` packages.

## Where to Look

| Task | Location | Notes |
|------|----------|-------|
| CLI dispatch | `cli.go` | `setup`, `doctor`, `update`, `uninstall` |
| Runtime setup | `setup.go`, `setup_input.go` | Preflight, flags, installers, ZeroClaw service |
| ZeroClaw policy/config | `setup_config.go` | Mention routing, OMO shell allowlist, SOUL.md, PATH |
| Process execution | `install.go` | Context-aware official installer and CLI calls |
| Readiness | `doctor.go` | Installed binaries, config, systemd service |
| Lifecycle | `lifecycle.go`, `manifest.go` | Updates, ownership, safe removal of managed files |
| Setup behavior tests | `cli_test.go` | CLI setup and missing-secret boundary |
| Lifecycle tests | `lifecycle_test.go` | Doctor, update preservation, uninstall preservation |

## Conventions

- Standard library only; do not add dependencies.
- Wrap errors with context using `fmt.Errorf("context: %w", err)`.
- Validate user-provided provider names before composing config paths.
- Use `zeroclaw config patch -` with JSON on stdin for credentials; do not pass secrets in argv.
- Keep OMO's executable available in the ZeroClaw systemd environment; login shell profiles are not loaded by systemd.
- Uninstall must preserve ZeroClaw/OMO config, secrets, memory, and workspaces.
- No commits unless the user explicitly requests one.

## Commands

```sh
go test ./...
go build -o prabu .
go vet ./...
```

The legacy source packages have their own package instructions under `bot/AGENTS.md` and `mcp/AGENTS.md`. They are not active runtime paths for Prabu.
