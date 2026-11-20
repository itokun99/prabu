package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runDoctor(ctx context.Context, args []string, streams cliStreams) error {
	options, help, err := parseLifecycleOptions(args, streams, lifecycleCommand{name: "doctor"})
	if err != nil {
		return err
	}
	if help {
		return nil
	}
	home, err := resolveHome(options.home)
	if err != nil {
		return err
	}
	manifest, err := readInstallManifest(home)
	if err != nil {
		return err
	}
	runner := newCommandRunner(home, streams)
	var problems []string
	if !executable(manifest.ZeroClawPath) {
		problems = append(problems, "ZeroClaw executable is missing")
	}
	if !executable(manifest.OMOPath) {
		problems = append(problems, "OMO Native executable is missing")
	}
	config, err := os.ReadFile(filepath.Join(home, ".zeroclaw", "config.toml"))
	if err != nil {
		problems = append(problems, "ZeroClaw config is missing")
	} else {
		for _, required := range []string{
			"mention_only = true",
			"allowed_commands = [\"omo\"]",
			"auto_approve = [\"shell\"]",
		} {
			if !strings.Contains(string(config), required) {
				problems = append(problems, "ZeroClaw config is missing "+required)
			}
		}
	}
	if !manifest.ServiceManaged {
		problems = append(problems, "ZeroClaw service is not managed by Prabu")
	}
	status, err := runner.output(ctx, commandSpec{
		operation: "check ZeroClaw systemd service",
		program:   "systemctl",
		args:      []string{"--user", "is-active", "zeroclaw"},
	})
	if err != nil || strings.TrimSpace(string(status)) != "active" {
		problems = append(problems, "ZeroClaw systemd service is not active")
	}
	dropIn, err := os.ReadFile(filepath.Join(home, ".config", "systemd", "user", "zeroclaw.service.d", "10-prabu-omo.conf"))
	if err != nil || !strings.Contains(string(dropIn), managedMarker) {
		problems = append(problems, "ZeroClaw service PATH override is missing")
	}
	instructions, err := os.ReadFile(filepath.Join(home, agentWorkspace, "SOUL.md"))
	if err != nil || len(strings.TrimSpace(string(instructions))) == 0 {
		problems = append(problems, "ZeroClaw OMO delegation instructions are missing")
	}
	if len(problems) > 0 {
		for _, problem := range problems {
			fmt.Fprintf(streams.out, "NOT READY: %s\n", problem)
		}
		return errors.New("prabu doctor found incomplete runtime setup")
	}
	fmt.Fprintln(streams.out, "READY: ZeroClaw and OMO Native are installed, Telegram routing is configured, and the ZeroClaw service is active.")
	fmt.Fprintln(streams.out, "OMO authentication is host-specific; use `omo login` if a delegated task reports missing credentials.")
	return nil
}
