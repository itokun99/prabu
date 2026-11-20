package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	streams := cliStreams{in: os.Stdin, out: os.Stdout, err: os.Stderr}
	if err := runCLI(ctx, os.Args[1:], streams); err != nil {
		fmt.Fprintf(streams.err, "prabu: %v\n", err)
		stop()
		os.Exit(1)
	}
}
