package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

func runSetup(ctx context.Context, args []string, streams cliStreams) error {
	options, err := parseSetupOptions(args, streams)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	if !validProviderType(options.provider) {
		return fmt.Errorf("invalid --provider %q; use a provider type such as ollama", options.provider)
	}
	if !validModelName(options.model) {
		return fmt.Errorf("invalid --model %q; model names may contain letters, numbers, '.', '_', ':', '/', '+', and '-'", options.model)
	}

	token := strings.TrimSpace(os.Getenv("PRABU_TELEGRAM_BOT_TOKEN"))
	if token == "" && (options.nonInteractive || !terminalInput(streams.in)) {
		return errors.New("Telegram bot token is required; set PRABU_TELEGRAM_BOT_TOKEN or run setup from a terminal for ZeroClaw's masked prompt")
	}
	if !options.yes {
		if !terminalInput(streams.in) {
			return errors.New("setup needs confirmation in a terminal; pass --yes to continue")
		}
		if err := confirmSetup(&streams); err != nil {
			return err
		}
	}

	home, err := resolveHome(options.home)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return fmt.Errorf("create home directory %s: %w", home, err)
	}
	previous, err := readInstallManifestIfPresent(home)
	if err != nil {
		return err
	}
	if previous.ZeroClawInstalled && filepath.Clean(previous.ZeroClawPath) != filepath.Join(home, ".cargo", "bin", "zeroclaw") {
		return errors.New("managed ZeroClaw install path changed; run prabu doctor before setup")
	}
	if previous.OMOInstalled && filepath.Clean(previous.OMOPath) != filepath.Join(home, ".local", "bin", "omo") {
		return errors.New("managed OMO install path changed; run prabu doctor before setup")
	}
	runner := newCommandRunner(home, streams)
	zeroClawPath := filepath.Join(home, ".cargo", "bin", "zeroclaw")
	zeroClaw, zeroClawInstalled, err := runner.ensureProgram(ctx, "zeroclaw", zeroClawPath, installerSpec{
		name:      "ZeroClaw",
		url:       zeroClawInstallerURL,
		shell:     "sh",
		shellArgs: []string{"--skip-quickstart"},
	})
	if err != nil {
		return err
	}
	omoPath := filepath.Join(home, ".local", "bin", "omo")
	omo, omoInstalled, err := runner.ensureProgram(ctx, "omo", omoPath, installerSpec{
		name:  "OMO Native",
		url:   omoInstallerURL,
		shell: "bash",
	})
	if err != nil {
		return err
	}
	manifest := installManifest{
		Version:           1,
		ZeroClawPath:      zeroClaw,
		ZeroClawInstalled: zeroClawInstalled || previous.ZeroClawInstalled,
		OMOPath:           omo,
		OMOInstalled:      omoInstalled || previous.OMOInstalled,
		ServiceManaged:    previous.ServiceManaged,
	}
	if err := writeInstallManifest(home, manifest); err != nil {
		return err
	}

	configPath := filepath.Join(home, ".zeroclaw", "config.toml")
	config := setupConfig{
		provider:    options.provider,
		model:       options.model,
		botToken:    token,
		providerKey: strings.TrimSpace(os.Getenv("PRABU_PROVIDER_API_KEY")),
	}
	if err := writeConfigSeed(configPath, config); err != nil {
		return err
	}
	patch, err := configPatch(config)
	if err != nil {
		return err
	}
	if err := runner.run(ctx, commandSpec{
		operation: "apply ZeroClaw configuration",
		program:   zeroClaw,
		args:      []string{"config", "patch", "-"},
		stdin:     bytes.NewReader(patch),
		quiet:     true,
	}); err != nil {
		return err
	}
	if token == "" {
		if err := runner.run(ctx, commandSpec{
			operation: "store Telegram bot token in ZeroClaw",
			program:   zeroClaw,
			args:      []string{"config", "set", secretConfigPath},
			inheritIn: true,
		}); err != nil {
			return err
		}
		activation, err := configActivationPatch()
		if err != nil {
			return err
		}
		if err := runner.run(ctx, commandSpec{
			operation: "enable ZeroClaw Telegram channel",
			program:   zeroClaw,
			args:      []string{"config", "patch", "-"},
			stdin:     bytes.NewReader(activation),
			quiet:     true,
		}); err != nil {
			return err
		}
	}

	if err := writeAgentInstructions(home); err != nil {
		return err
	}
	if err := runner.run(ctx, commandSpec{
		operation: "install ZeroClaw user service",
		program:   zeroClaw,
		args:      []string{"service", "install"},
		inheritIn: true,
	}); err != nil {
		return err
	}
	manifest.ServiceManaged = true
	if err := writeInstallManifest(home, manifest); err != nil {
		return err
	}
	if err := writeServicePathDropIn(home, zeroClaw, omo); err != nil {
		return err
	}
	if err := runner.run(ctx, commandSpec{
		operation: "reload user systemd configuration",
		program:   "systemctl",
		args:      []string{"--user", "daemon-reload"},
	}); err != nil {
		return err
	}
	if err := runner.run(ctx, commandSpec{
		operation: "start ZeroClaw user service",
		program:   zeroClaw,
		args:      []string{"service", "start"},
		inheritIn: true,
	}); err != nil {
		return err
	}
	account, err := user.Current()
	if err != nil {
		return fmt.Errorf("identify service user for systemd lingering: %w", err)
	}
	if err := runner.run(ctx, commandSpec{
		operation: "enable systemd lingering for ZeroClaw",
		program:   "sudo",
		args:      []string{"loginctl", "enable-linger", account.Username},
		inheritIn: true,
	}); err != nil {
		return err
	}

	fmt.Fprintln(streams.out, "Prabu setup complete.")
	fmt.Fprintln(streams.out, "ZeroClaw will answer direct messages, group mentions, and topic replies.")
	fmt.Fprintf(streams.out, "OMO execution is available through %s.\n", omo)
	fmt.Fprintln(streams.out, "If OMO is not authenticated on this host, run `omo login` before delegating work.")
	fmt.Fprintln(streams.out, "Authorize your Telegram account with ZeroClaw's pairing flow before using the bot in groups.")
	return nil
}
