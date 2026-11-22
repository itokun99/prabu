# prabu

**Personal Assistant / AI setup for your homelab**

Prabu bootstraps a personal AI assistant on a Linux homelab. It installs and configures [ZeroClaw](https://github.com/zeroclaw-labs/zeroclaw) as the Telegram-facing assistant and [OMO Native](https://github.com/code-yeongyu/oh-my-openagent) as its delegated execution harness.

```text
You in Telegram ──DM or mention──> ZeroClaw ──shell: omo -p──> OMO Native
                                      └──────── final result ────────> You
```

ZeroClaw owns Telegram polling, authorization, chat sessions, and forum-topic reply routing. OMO runs execution tasks when ZeroClaw delegates through its shell tool. Prabu does not run a Telegram bot, webhook, OpenCode server, or MCP bridge.

## What setup does

- Installs ZeroClaw and OMO Native using their official installers.
- Configures ZeroClaw with a model provider, a Telegram channel, and an agent instruction to delegate execution to OMO using `omo -p --approve --no-session`.
- Enables replies to direct messages and group mentions. Telegram forum-topic replies remain in their originating topic; `/topic` lifecycle commands are not part of Prabu.
- Keeps the shell tool restricted to the `omo` executable, runs it in ZeroClaw's workspace, and gives it a bounded 30-minute timeout.
- Installs and starts ZeroClaw's user-scoped systemd service, adds OMO's install directory to its `PATH`, and enables systemd lingering.
- Stores the Telegram token through ZeroClaw's secret-aware config interface; the token is not written to `config.toml`.

Telegram uses the Bot API's long-polling `getUpdates` method. It needs outbound network access to Telegram, but no public URL, webhook, reverse proxy, or inbound port. Only one process should poll a bot token at a time.

## Requirements

- Linux with a systemd user service and `sudo` access for `loginctl enable-linger`.
- Go 1.26 or newer to build Prabu.
- A Telegram bot token from [@BotFather](https://t.me/botfather).
- A ZeroClaw model provider. The default example is a local Ollama model; hosted providers may need `PRABU_PROVIDER_API_KEY`.
- OMO Native model authentication configured on the homelab. Use `omo login` when needed; Prabu does not copy laptop credentials to the server.

## Get the source

```sh
git clone https://github.com/itokun99/prabu.git
cd prabu
```

## Build and configure

From a Prabu checkout:

```sh
go build -o prabu .
```

For a local Ollama installation, set the Telegram token in the environment and run:

```sh
export PRABU_TELEGRAM_BOT_TOKEN='your-bot-token'
./prabu setup --provider ollama --model llama3.1 --yes
```

The default provider and model are `ollama` and `llama3.1`. To use another provider, pass its ZeroClaw provider type and model name. If that provider needs an API key, set `PRABU_PROVIDER_API_KEY` before setup; ZeroClaw receives secrets through its JSON Patch stdin interface, which stores them in its managed secret state. Keep secret values out of shell history and logs.

For interactive setup, omit `PRABU_TELEGRAM_BOT_TOKEN`; ZeroClaw prompts for it using masked input. `--non-interactive` requires the token environment variable. `--home <path>` selects a different home directory, useful for isolated installs and testing.

After setup, authenticate OMO if needed:

```sh
omo login
omo doctor --models --json
```

Send the bot a private message and complete ZeroClaw's pairing flow before using it in a group. Then add the bot to the group and mention it, or reply to one of its messages. If the group uses forum topics, ZeroClaw routes the reply into the same topic. Direct messages are always processed; unmentioned group messages are ignored.

## ZeroClaw configuration

Prabu manages the `prabu` provider, risk profile, agent, and Telegram channel aliases in `~/.zeroclaw/config.toml` (schema version 4). The generated configuration uses a supervised profile, workspace-only file access, and a minimal ZeroClaw tool set that includes the shell tool. The shell allowlist contains only `omo`; shell delegation is auto-approved so the Telegram assistant can wait for OMO without an extra approval round-trip. Other tools continue to follow ZeroClaw's policy.

The channel has `mention_only = true` and retains ZeroClaw's default per-user session scope, including within forum topics. The Telegram bot token is set through ZeroClaw's secret-aware config path and is not placed in the TOML file.

The agent's `SOUL.md` instructs ZeroClaw to delegate execution tasks through OMO and to report results only after the subprocess finishes. Ordinary conversational questions can be answered directly. The OMO Native install path is added to the ZeroClaw systemd user service's `PATH`, because systemd does not load an interactive shell profile.

## Lifecycle commands

```sh
./prabu doctor --home "$HOME"
./prabu update --home "$HOME" --yes
./prabu uninstall --home "$HOME" --yes
```

`doctor` checks the managed binaries, ZeroClaw config, OMO instruction file, systemd service override, and active user service. It does not verify that OMO provider credentials work; use `omo doctor` or run a small delegated task to check authentication.

`update` uses ZeroClaw's updater, refreshes OMO Native using its official installer or package update command, then restarts the managed ZeroClaw service. `uninstall --yes` removes the Prabu-managed service and only binaries Prabu installed. It preserves `~/.zeroclaw`, `~/.omo`, credentials, agent workspaces, and memory.

## Repository structure

```text
main.go          CLI entry point
cli.go           command dispatch and flags
setup.go         ZeroClaw/OMO setup orchestration
setup_input.go   setup flags, validation, and confirmation
setup_config.go  ZeroClaw config patch, SOUL.md, and systemd PATH override
install.go       official installer and subprocess handling
manifest.go      managed install state
doctor.go        runtime readiness checks
lifecycle.go     update and uninstall
```

## Development

```sh
go test ./...
go build -o prabu .
go vet ./...
```

Prabu itself uses only the Go standard library.

## Licensing

This repository is licensed under CC BY-NC 4.0; see [LICENSE](LICENSE). ZeroClaw and OMO are installed separately and retain their own licenses. ZeroClaw is MIT OR Apache-2.0. OMO's repository uses the Sustainable Use License (SUL-1.0); review its terms before redistributing OMO or offering it as part of a paid or hosted service.
