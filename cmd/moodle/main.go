// Command moodle is the Moodle client for students, scripts and agents.
//
// This file does one thing: create the root context and hand over to the
// composition root. All logic lives under internal/.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/KoukeNeko/moodle-cli/internal/bootstrap"
)

// Injected at link time by the Makefile.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = ""
)

func main() {
	// The only root context in the program. Ctrl-C and SIGTERM cancel it, and
	// every downstream call receives it as its first argument.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(bootstrap.Run(ctx, bootstrap.Build{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
	}, os.Args[1:]))
}
