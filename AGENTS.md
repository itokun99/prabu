# PROJECT KNOWLEDGE BASE

**Generated:** 2026-11-21T17:53:36Z
**Commit:** 9004da8
**Branch:** main

## OVERVIEW

Prabu is a Go 1.26 standard-library-only CLI that bootstraps a homelab AI stack: it installs/configures ZeroClaw (the Telegram-facing assistant) and OMO Native (its delegated executor) on a user-scoped Linux/systemd host.

## STRUCTURE

```text
prabu/
├── main.go          # Entry point; signal-aware context → runCLI
├── cli.go           # Command dispatch, usage, shared flag parsing
├── setup.go         # setup: install ZeroClaw/OMO, seed config, start service
├── setup_input.go   # setup flags, validation, terminal token prompt
├── setup_config.go  # ZeroClaw config seed/patch, SOUL.md, systemd PATH drop-in
├── install.go       # Installer download, subprocess runner, child env sanitizing
├── manifest.go      # ~/.config/prabu/install.json read/write (Version must be 1)
├── doctor.go        # doctor: readiness checks against manifest + live files
├── lifecycle.go     # update / uninstall with ownership checks
├── *_test.go        # sandbox-HOME tests with stub executables
├── env.template     # PRABU_TELEGRAM_BOT_TOKEN, PRABU_PROVIDER_API_KEY (reference only)
└── README.md        # User-facing guide
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Add/modify a subcommand | `cli.go` (`runCLI` switch) | unknown command → error naming `prabu --help` |
| Setup flow order | `setup.go` | manifest check → ensureProgram ×2 → config seed → service |
| Flags for a command | `setup_input.go`, `lifecycle.go` | `parseSetupOptions` / `parseLifecycleOptions` |
| ZeroClaw config contents | `setup_config.go` (`configSeed`) | exact TOML emitted; patch ops for secrets |
| Credential routing | `setup.go` + `setup_config.go` | token → stdin of `zeroclaw config patch -` |
| PATH for systemd service | `setup_config.go:writeServicePathDropIn` | drop-in at `~/.config/systemd/user/zeroclaw.service.d/10-prabu-omo.conf` |
| Install state | `manifest.go` | `~/.config/prabu/install.json`, mode 0600 |
| Readiness checks | `doctor.go` | checks binaries, config strings, service, drop-in, SOUL.md |
| Update/uninstall safety | `lifecycle.go` | refuses to touch paths outside the manifest |

## CODE MAP

| Symbol | Type | Location | Role |
|--------|------|----------|------|
| `main` | func | main.go | signals (SIGINT/SIGTERM) → `runCLI` |
| `runCLI` | func | cli.go | dispatch: setup/doctor/update/uninstall/version |
| `cliStreams` | type | cli.go | in/out/err; everything takes this, never raw os.Stdout |
| `runSetup` | func | setup.go | full bootstrap orchestration |
| `parseSetupOptions` | func | setup_input.go | flags: `--home --provider --model --yes --non-interactive` |
| `confirmSetup` | func | setup_input.go | interactive `[y/N]` gate |
| `configSeed` | func | setup_config.go | builds base config.toml (schema_version 4) |
| `applyConfigPatch` | func | setup_config.go | JSON patch ops → `zeroclaw config patch -` stdin |
| `writeAgentInstructions` | func | setup_config.go | managed SOUL.md block with OMO delegation rules |
| `writeServicePathDropIn` | func | setup_config.go | systemd user service PATH override |
| `commandRunner` | type | install.go | context-aware subprocess wrapper for installers/CLI |
| `ensureProgram` | func | install.go | find on PATH else run official installer |
| `installManifest` | type | manifest.go | ZeroClawPath/OMOPath/ServiceManaged, versioned |
| `readInstallManifest` | func | manifest.go | errors on missing/undecodable/wrong-version |
| `runDoctor` | func | doctor.go | NOT READY list or READY summary |
| `runUpdate` / `runUninstall` | func | lifecycle.go | re-run installers / remove managed files only |

## CONVENTIONS

- Standard library only; no new Go dependencies.
- Errors: `fmt.Errorf("context: %w", err)`; multiple failures join with `errors.Join`.
- User input (provider/model) validated against a strict charset before composing paths.
- All I/O flows through `cliStreams`; subprocess env is filtered (secrets stripped).
- Names are exact ZeroClaw identifiers: provider/agent/channel key `prabu`, alias `telegram.prabu`.

## ANTI-PATTERNS (THIS PROJECT)

- NEVER enable or call ZeroClaw's `opencode_cli` tool; no parallel Telegram bridge.
- NEVER write a Telegram token to TOML, logs, or argv; route it via stdin to `zeroclaw config patch -` (JSON patch path `/channels/telegram/prabu/bot_token`).
- NEVER remove ZeroClaw/OMO files outside the manifest-owned paths (`~/.cargo/bin/zeroclaw`, `~/.local/bin/omo`, the `10-prabu-omo.conf` drop-in).
- NEVER break uninstall preservation: `~/.zeroclaw` config, secrets, memory, and agent workspaces survive.
- No commits unless the user explicitly requests one.

## UNIQUE STYLES

- Tests stub the world: sandbox `HOME`, fake `zeroclaw`/`omo`/`systemctl`/`sudo`/`loginctl` executables on a controlled `PATH`, and a `PRABU_CALL_LOG` file recording every stub invocation (`cli_test.go`, `lifecycle_test.go`).
- The config contract is asserted as exact strings, e.g. `allowed_commands = ["omo"]`, `auto_approve = ["shell"]`, `mention_only = true`; `cli_test.go:68` asserts `opencode_cli` never appears in the emitted config.
- Managed files carry the marker `# Managed by prabu. Do not edit this file directly.`

## COMMANDS

```bash
go test ./...            # full suite (~35s), sandboxed, no host changes
go build -o prabu .
go vet ./...
go run . <subcommand>    # source-tree invocation
```

## NOTES

- Run targets a Linux/systemd host; on macOS only build/vet/test run meaningfully.
- OMO auth is host-specific; `prabu doctor` does not verify it (see its READY output).
- Secrets enter via `PRABU_TELEGRAM_BOT_TOKEN` / `PRABU_PROVIDER_API_KEY` (env or terminal prompt); `env.template` is documentation, never loaded.
