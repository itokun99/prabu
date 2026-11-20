package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	zeroClawInstallerURL = "https://raw.githubusercontent.com/zeroclaw-labs/zeroclaw/master/install.sh"
	omoInstallerURL      = "https://get.omo.dev/install.sh"
)

type commandRunner struct {
	home    string
	path    string
	env     []string
	streams cliStreams
}

type commandSpec struct {
	operation string
	program   string
	args      []string
	stdin     io.Reader
	inheritIn bool
	quiet     bool
}

type installerSpec struct {
	name      string
	url       string
	shell     string
	shellArgs []string
}

func newCommandRunner(home string, streams cliStreams) commandRunner {
	basePath := os.Getenv("PATH")
	searchPath := strings.Join([]string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".cargo", "bin"),
		basePath,
	}, string(os.PathListSeparator))
	env := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "HOME", "PATH", "PRABU_TELEGRAM_BOT_TOKEN", "PRABU_PROVIDER_API_KEY":
			continue
		default:
			env = append(env, entry)
		}
	}
	env = append(env, "HOME="+home, "PATH="+searchPath)
	return commandRunner{home: home, path: searchPath, env: env, streams: streams}
}

func (runner commandRunner) lookup(program string) (string, error) {
	for _, directory := range filepath.SplitList(runner.path) {
		if directory == "" {
			continue
		}
		candidate := filepath.Join(directory, program)
		if executable(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s is not installed or is not on PATH", program)
}

func (runner commandRunner) run(ctx context.Context, spec commandSpec) error {
	program := spec.program
	if !filepath.IsAbs(program) {
		resolved, err := runner.lookup(program)
		if err != nil {
			return fmt.Errorf("%s: %w", spec.operation, err)
		}
		program = resolved
	}

	command := exec.CommandContext(ctx, program, spec.args...)
	command.Env = runner.env
	if spec.stdin != nil {
		command.Stdin = spec.stdin
	} else if spec.inheritIn {
		command.Stdin = runner.streams.in
	}
	if spec.quiet {
		var stdout, stderr bytes.Buffer
		command.Stdout = &stdout
		command.Stderr = &stderr
	} else {
		command.Stdout = runner.streams.out
		command.Stderr = runner.streams.err
	}
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s: %w", spec.operation, err)
	}
	return nil
}

func (runner commandRunner) output(ctx context.Context, spec commandSpec) ([]byte, error) {
	program := spec.program
	if !filepath.IsAbs(program) {
		resolved, err := runner.lookup(program)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", spec.operation, err)
		}
		program = resolved
	}

	command := exec.CommandContext(ctx, program, spec.args...)
	command.Env = runner.env
	var stdout bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = runner.streams.err
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("%s: %w", spec.operation, err)
	}
	return stdout.Bytes(), nil
}

func (runner commandRunner) ensureProgram(ctx context.Context, name, preferred string, installer installerSpec) (string, bool, error) {
	if executable(preferred) {
		return preferred, false, nil
	}
	if path, err := runner.lookup(name); err == nil {
		return path, false, nil
	}
	if err := runner.install(ctx, installer); err != nil {
		return "", false, err
	}
	if executable(preferred) {
		return preferred, true, nil
	}
	path, err := runner.lookup(name)
	if err != nil {
		return "", false, fmt.Errorf("%s installer completed but the executable is missing: %w", installer.name, err)
	}
	return path, true, nil
}

func (runner commandRunner) install(ctx context.Context, installer installerSpec) error {
	script, err := runner.output(ctx, commandSpec{
		operation: "download " + installer.name + " installer",
		program:   "curl",
		args:      []string{"-fsSL", installer.url},
	})
	if err != nil {
		return err
	}
	args := append([]string{"-s", "--"}, installer.shellArgs...)
	return runner.run(ctx, commandSpec{
		operation: "run " + installer.name + " installer",
		program:   installer.shell,
		args:      args,
		stdin:     bytes.NewReader(script),
	})
}

func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}
