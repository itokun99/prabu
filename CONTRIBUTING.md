# Contributing

Contributions should improve Prabu's homelab bootstrap experience and preserve its ZeroClaw-to-OMO architecture.

## Ground Rules

- Prabu is standard-library-only Go. Do not add external Go dependencies without explicit approval.
- ZeroClaw owns Telegram channels, including direct messages, group mentions, authorization, and forum-topic routing.
- OMO Native is invoked through the ZeroClaw shell tool with `omo -p`; do not add `opencode_cli` or a parallel Telegram bridge.
- Keep secrets out of configuration TOML, command arguments, logs, and tests. Use ZeroClaw's secret-aware config interface.
- `uninstall` must preserve ZeroClaw/OMO user configuration, credentials, memory, and workspaces.
- This repository is licensed CC BY-NC 4.0. Read [LICENSE](LICENSE) before contributing.

## Before Submitting

```sh
go test ./...
go build -o prabu .
go vet ./...
```

Add tests for new CLI behavior and preserve existing tests. For changes to installers or lifecycle commands, include an isolated-home scenario with controlled upstream executables; never run installer or service changes on a developer host as part of tests.

Keep user-facing documentation in English. Do not commit unless the repository owner asks for it.
