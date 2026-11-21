package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type setupOptions struct {
	home           string
	provider       string
	model          string
	yes            bool
	nonInteractive bool
}

func parseSetupOptions(args []string, streams cliStreams) (setupOptions, error) {
	var options setupOptions
	flags := newFlagSet("setup", streams)
	flags.StringVar(&options.home, "home", "", "home directory to configure")
	flags.StringVar(&options.provider, "provider", "ollama", "ZeroClaw model provider type")
	flags.StringVar(&options.model, "model", "llama3.1", "model name for the ZeroClaw agent")
	flags.BoolVar(&options.yes, "yes", false, "confirm installation and service setup")
	flags.BoolVar(&options.nonInteractive, "non-interactive", false, "require all secrets through environment variables")
	if err := parseFlags(flags, args); err != nil {
		return setupOptions{}, err
	}
	return options, nil
}

func validProviderType(provider string) bool {
	if provider == "" {
		return false
	}
	for index, char := range provider {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_' || char == '-' {
			if index == 0 && char >= '0' && char <= '9' {
				return false
			}
			continue
		}
		return false
	}
	return true
}

func validModelName(model string) bool {
	if model == "" {
		return false
	}
	for _, char := range model {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' || strings.ContainsRune("._:/+-", char) {
			continue
		}
		return false
	}
	return true
}

// readTerminalToken asks the operator for the ZeroClaw bot token on the given
// terminal and reads one line. The token travels only via stdin here and later
// via a JSON `config patch -` op; it never lands in argv, config, or logs. The
// caller must have verified terminalInput first.
func readTerminalToken(input io.Reader, out io.Writer) (string, error) {
	fmt.Fprint(out, "Telegram bot token: ")
	reader, ok := input.(*bufio.Reader)
	if !ok {
		reader = bufio.NewReader(input)
	}
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read Telegram bot token: %w", err)
	}
	token := strings.TrimSpace(line)
	if token == "" {
		return "", errors.New("Telegram bot token must not be empty")
	}
	return token, nil
}

func terminalInput(input io.Reader) bool {
	file, ok := input.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	nullInfo, err := os.Stat(os.DevNull)
	if err != nil {
		return false
	}
	return !os.SameFile(info, nullInfo)
}

func confirmSetup(streams *cliStreams) error {
	fmt.Fprint(streams.out, "Install ZeroClaw and OMO Native, configure Telegram, and start the user service? [y/N] ")
	reader, ok := streams.in.(*bufio.Reader)
	if !ok {
		reader = bufio.NewReader(streams.in)
		streams.in = reader
	}
	answer, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read setup confirmation: %w", err)
	}
	if strings.ToLower(strings.TrimSpace(answer)) != "y" && strings.ToLower(strings.TrimSpace(answer)) != "yes" {
		return errors.New("setup cancelled")
	}
	return nil
}
