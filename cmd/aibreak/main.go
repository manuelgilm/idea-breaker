package main

import (
	"fmt"
	"os"

	"aibreak/internal/app"
	"aibreak/internal/cli"
	"aibreak/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	if p := dbFlag(os.Args[1:]); p != "" {
		cfg.DBPath = p
	}

	svc, store, err := app.Build(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer store.Close()

	if err := cli.New(svc).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// dbFlag extracts a --db path override from raw args before cobra parsing,
// because the store must be opened with the final path.
func dbFlag(args []string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--db" {
			return args[i+1]
		}
	}
	return ""
}
