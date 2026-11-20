package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type cliStreams struct {
	in  io.Reader
	out io.Writer
	err io.Writer
}

func runCLI(ctx context.Context, args []string, streams cliStreams) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printUsage(streams.out)
		return nil
	}

	switch args[0] {
	case "setup":
		return runSetup(ctx, args[1:], streams)
	case "doctor":
		return runDoctor(ctx, args[1:], streams)
	case "update":
		return runUpdate(ctx, args[1:], streams)
	case "uninstall":
		return runUninstall(ctx, args[1:], streams)
	case "version", "--version":
		fmt.Fprintln(streams.out, "prabu dev")
		return nil
	default:
		return fmt.Errorf("unknown command %q; run prabu --help", args[0])
	}
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, `prabu — Personal Assistant / AI setup for your homelab

Usage:
  prabu setup --provider <type> --model <name> [--home <path>] [--yes] [--non-interactive]
  prabu doctor [--home <path>]
  prabu update --yes [--home <path>]
  prabu uninstall --yes [--home <path>]

setup installs ZeroClaw and OMO Native, configures Telegram mention/topic
routing, and delegates execution through the OMO shell command `+"`omo -p`"+`.`)
}

func newFlagSet(name string, streams cliStreams) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(streams.err)
	flags.Usage = func() {
		printUsage(streams.out)
	}
	return flags
}

func parseFlags(flags *flag.FlagSet, args []string) error {
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return flag.ErrHelp
		}
		return err
	}
	if len(flags.Args()) > 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	return nil
}

func resolveHome(path string) (string, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find user home: %w", err)
		}
		path = home
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve home %q: %w", path, err)
	}
	return filepath.Clean(absolute), nil
}
