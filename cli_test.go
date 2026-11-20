package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type prabuCLI struct {
	home    string
	bin     string
	callLog string
}

type prabuInvocation struct {
	token string
	args  []string
}

func TestSetup_writesMentionRoutingAndOMOShellConfig(t *testing.T) {
	cli := newPrabuCLI(t)
	output, err := cli.run(t, prabuInvocation{
		token: "test-token",
		args: []string{
			"setup",
			"--home", cli.home,
			"--provider", "ollama",
			"--model", "llama3.1",
			"--yes",
		},
	})
	if err != nil {
		t.Fatalf("prabu setup failed: %v\n%s", err, output)
	}

	configPath := filepath.Join(cli.home, ".zeroclaw", "config.toml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read ZeroClaw config: %v", err)
	}
	for _, field := range []string{
		"schema_version = 4",
		"[risk_profiles.prabu]",
		"allowed_commands = [\"omo\"]",
		"auto_approve = [\"shell\"]",
		"always_ask = []",
		"[agents.prabu]",
		"channels = [\"telegram.prabu\"]",
		"[channels.telegram.prabu]",
		"enabled = true",
		"mention_only = true",
		"per_user_session = true",
		"[tools]",
		"optional = []",
		"[shell_tool]",
		"timeout_secs = 1800",
	} {
		if !bytes.Contains(config, []byte(field)) {
			t.Errorf("ZeroClaw config missing machine setting %q:\n%s", field, config)
		}
	}
	if bytes.Contains(config, []byte("opencode_cli")) {
		t.Error("ZeroClaw config must not enable or configure opencode_cli")
	}
	if bytes.Contains(config, []byte("test-token")) {
		t.Error("Telegram token must not be written to config.toml")
	}
	calls, err := os.ReadFile(cli.callLog)
	if err != nil {
		t.Fatalf("read external command log: %v", err)
	}
	if bytes.Contains(calls, []byte("test-token")) {
		t.Error("Telegram token must not be passed in a child process argument")
	}

	soulPath := filepath.Join(cli.home, ".zeroclaw", "agents", "prabu", "workspace", "SOUL.md")
	soul, err := os.ReadFile(soulPath)
	if err != nil {
		t.Fatalf("read OMO delegation instructions: %v", err)
	}
	if len(bytes.TrimSpace(soul)) == 0 {
		t.Fatal("ZeroClaw agent workspace must contain its OMO delegation instructions")
	}

	dropInPath := filepath.Join(cli.home, ".config", "systemd", "user", "zeroclaw.service.d", "10-prabu-omo.conf")
	dropIn, err := os.ReadFile(dropInPath)
	if err != nil {
		t.Fatalf("read ZeroClaw service PATH drop-in: %v", err)
	}
	if !bytes.Contains(dropIn, []byte(".local/bin")) {
		t.Fatalf("ZeroClaw service PATH does not include OMO install directory:\n%s", dropIn)
	}
}

func TestSetup_withoutTokenPreservesExistingConfig(t *testing.T) {
	cli := newPrabuCLI(t)
	configPath := filepath.Join(cli.home, ".zeroclaw", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	existing := []byte("# user-owned ZeroClaw config\n[custom]\nvalue = \"keep\"\n")
	if err := os.WriteFile(configPath, existing, 0o600); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	output, err := cli.run(t, prabuInvocation{
		args: []string{
			"setup",
			"--home", cli.home,
			"--provider", "ollama",
			"--model", "llama3.1",
			"--yes",
		},
	})
	if err == nil {
		t.Fatalf("prabu setup without a token succeeded:\n%s", output)
	}
	if !strings.Contains(output, "PRABU_TELEGRAM_BOT_TOKEN") {
		t.Fatalf("missing-token error should name PRABU_TELEGRAM_BOT_TOKEN:\n%s", output)
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read existing config after setup: %v", err)
	}
	if !bytes.Equal(got, existing) {
		t.Fatalf("setup changed existing config before validating the token:\n%s", got)
	}

	calls, err := os.ReadFile(cli.callLog)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read external command log: %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("setup ran external commands before validating the token:\n%s", calls)
	}
}

func newPrabuCLI(t *testing.T) prabuCLI {
	t.Helper()
	home := t.TempDir()
	bin := t.TempDir()
	callLog := filepath.Join(t.TempDir(), "commands.log")

	for name, body := range map[string]string{
		"zeroclaw": `#!/bin/sh
set -eu
printf 'zeroclaw %s %s %s\n' "${1:-}" "${2:-}" "${3:-}" >> "$PRABU_CALL_LOG"
if [ "${1:-}" = "config" ] && [ "${2:-}" = "patch" ]; then
  patch="$(cat)"
  case "$patch" in
    *'"path":"/channels/telegram/prabu/bot_token"'*'"value":"test-token"'*) ;;
    *) printf 'missing Telegram secret patch\n' >&2; exit 24 ;;
  esac
  case "$patch" in
    *'"path":"/channels/telegram/prabu/enabled"'*'"value":true'*) ;;
    *) printf 'missing channel activation patch\n' >&2; exit 25 ;;
  esac
  config="$HOME/.zeroclaw/config.toml"
  sed 's/^enabled = false$/enabled = true/' "$config" > "$config.tmp"
  mv "$config.tmp" "$config"
fi
if [ "${1:-}" = "update" ]; then
  printf 'updated\n' > "$HOME/.prabu-zero-updated"
fi
if [ "${1:-}" = "service" ] && [ "${2:-}" = "uninstall" ]; then
  rm -f "$HOME/.config/systemd/user/zeroclaw.service"
fi
`,
		"omo": `#!/bin/sh
set -eu
printf 'omo %s\n' "${1:-}" >> "$PRABU_CALL_LOG"
`,
		"systemctl": `#!/bin/sh
set -eu
case " $* " in
  *" is-active "*) printf 'active\n'; exit 0 ;;
esac
printf 'systemctl %s %s\n' "${1:-}" "${2:-}" >> "$PRABU_CALL_LOG"
`,
		"sudo": `#!/bin/sh
set -eu
printf 'sudo %s %s\n' "${1:-}" "${2:-}" >> "$PRABU_CALL_LOG"
`,
		"loginctl": `#!/bin/sh
set -eu
printf 'loginctl %s %s\n' "${1:-}" "${2:-}" >> "$PRABU_CALL_LOG"
`,
		"curl": `#!/bin/sh
set -eu
case "$*" in
  *get.omo.dev*) printf 'OMO_INSTALLER\n' ;;
  *zeroclaw-labs*) printf 'ZEROCLAW_INSTALLER\n' ;;
  *) exit 30 ;;
esac
`,
		"bash": `#!/bin/sh
set -eu
script="$(cat)"
case "$script" in
  *OMO_INSTALLER*) printf 'updated\n' > "$HOME/.prabu-omo-updated" ;;
  *) exit 31 ;;
esac
`,
		"sh": `#!/bin/sh
set -eu
script="$(cat)"
case "$script" in
  *ZEROCLAW_INSTALLER*) printf 'installed\n' > "$HOME/.prabu-zero-installed" ;;
  *) exit 32 ;;
esac
`,
	} {
		path := filepath.Join(bin, name)
		if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
			t.Fatalf("write %s stub: %v", name, err)
		}
	}

	return prabuCLI{home: home, bin: bin, callLog: callLog}
}

func (cli prabuCLI) run(t *testing.T, invocation prabuInvocation) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := append([]string{"run", "."}, invocation.args...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Env = cli.environment(invocation.token)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (cli prabuCLI) environment(token string) []string {
	env := make([]string, 0, len(os.Environ())+4)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "HOME", "PATH", "PRABU_CALL_LOG", "PRABU_TELEGRAM_BOT_TOKEN":
			continue
		default:
			env = append(env, entry)
		}
	}
	env = append(env,
		"HOME="+cli.home,
		"PATH="+cli.bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"PRABU_CALL_LOG="+cli.callLog,
	)
	if token != "" {
		env = append(env, "PRABU_TELEGRAM_BOT_TOKEN="+token)
	}
	return env
}
