package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type lifecycleOptions struct {
	home string
	yes  bool
}

func runUpdate(ctx context.Context, args []string, streams cliStreams) error {
	options, help, err := parseLifecycleOptions(args, streams, lifecycleCommand{name: "update", confirm: true})
	if err != nil || help {
		return err
	}
	if !options.yes {
		return errors.New("pass --yes to update managed runtimes")
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
	zeroClaw, err := runner.lookup("zeroclaw")
	if err != nil {
		return err
	}
	if filepath.Clean(zeroClaw) != filepath.Clean(manifest.ZeroClawPath) {
		return errors.New("ZeroClaw executable differs from Prabu's install manifest")
	}
	omo, err := runner.lookup("omo")
	if err != nil {
		return err
	}
	if filepath.Clean(omo) != filepath.Clean(manifest.OMOPath) {
		return errors.New("OMO Native executable differs from Prabu's install manifest")
	}
	if err := runner.run(ctx, commandSpec{
		operation: "update ZeroClaw",
		program:   zeroClaw,
		args:      []string{"update", "--force"},
		inheritIn: true,
	}); err != nil {
		return err
	}
	if manifest.OMOInstalled {
		if err := runner.install(ctx, installerSpec{name: "OMO Native", url: omoInstallerURL, shell: "bash"}); err != nil {
			return err
		}
	} else if err := runner.run(ctx, commandSpec{
		operation: "update OMO Native",
		program:   omo,
		args:      []string{"update"},
		inheritIn: true,
	}); err != nil {
		return err
	}
	if manifest.ServiceManaged {
		if err := runner.run(ctx, commandSpec{
			operation: "restart ZeroClaw service after update",
			program:   zeroClaw,
			args:      []string{"service", "restart"},
			inheritIn: true,
		}); err != nil {
			return err
		}
	}
	fmt.Fprintln(streams.out, "Updated ZeroClaw and OMO Native; user config and workspace were preserved.")
	return nil
}

func runUninstall(ctx context.Context, args []string, streams cliStreams) error {
	options, help, err := parseLifecycleOptions(args, streams, lifecycleCommand{name: "uninstall", confirm: true})
	if err != nil || help {
		return err
	}
	if !options.yes {
		return errors.New("uninstall removes managed binaries and services; pass --yes to confirm")
	}
	home, err := resolveHome(options.home)
	if err != nil {
		return err
	}
	manifest, err := readInstallManifest(home)
	if err != nil {
		return err
	}
	if manifest.ZeroClawInstalled && filepath.Clean(manifest.ZeroClawPath) != filepath.Join(home, ".cargo", "bin", "zeroclaw") {
		return errors.New("refusing to remove ZeroClaw outside Prabu's managed install path")
	}
	if manifest.OMOInstalled && filepath.Clean(manifest.OMOPath) != filepath.Join(home, ".local", "bin", "omo") {
		return errors.New("refusing to remove OMO Native outside Prabu's managed install path")
	}
	runner := newCommandRunner(home, streams)
	if manifest.ServiceManaged {
		zeroClaw, err := runner.lookup("zeroclaw")
		if err != nil {
			return err
		}
		if filepath.Clean(zeroClaw) != filepath.Clean(manifest.ZeroClawPath) {
			return errors.New("ZeroClaw executable differs from Prabu's install manifest")
		}
		for _, action := range []string{"stop", "uninstall"} {
			if err := runner.run(ctx, commandSpec{
				operation: "remove ZeroClaw service",
				program:   zeroClaw,
				args:      []string{"service", action},
				inheritIn: true,
			}); err != nil {
				return err
			}
		}
	}
	if err := removeServicePathDropIn(home); err != nil {
		return err
	}
	if manifest.ServiceManaged {
		if err := runner.run(ctx, commandSpec{
			operation: "reload user systemd configuration",
			program:   "systemctl",
			args:      []string{"--user", "daemon-reload"},
		}); err != nil {
			return err
		}
	}
	if manifest.ZeroClawInstalled {
		if err := removeIfPresent(manifest.ZeroClawPath); err != nil {
			return err
		}
	}
	if manifest.OMOInstalled {
		if err := removeIfPresent(manifest.OMOPath); err != nil {
			return err
		}
	}
	if err := removeIfPresent(installManifestPath(home)); err != nil {
		return err
	}
	fmt.Fprintln(streams.out, "Removed Prabu-managed services and binaries; ZeroClaw and OMO user data were preserved.")
	return nil
}

type lifecycleCommand struct {
	name    string
	confirm bool
}

func parseLifecycleOptions(args []string, streams cliStreams, command lifecycleCommand) (lifecycleOptions, bool, error) {
	var options lifecycleOptions
	flags := newFlagSet(command.name, streams)
	flags.StringVar(&options.home, "home", "", "home directory to inspect")
	if command.confirm {
		flags.BoolVar(&options.yes, "yes", false, "confirm this operation")
	}
	if err := parseFlags(flags, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return lifecycleOptions{}, true, nil
		}
		return lifecycleOptions{}, false, err
	}
	return options, false, nil
}

func removeServicePathDropIn(home string) error {
	path := filepath.Join(home, ".config", "systemd", "user", "zeroclaw.service.d", "10-prabu-omo.conf")
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read ZeroClaw service override: %w", err)
	}
	if !strings.Contains(string(content), managedMarker) {
		return nil
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove Prabu ZeroClaw service override: %w", err)
	}
	return nil
}

func removeIfPresent(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}
