package main

import (
	"fmt"
	"os"
	"strings"

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
	if dbPath, rest := extractDBFlag(os.Args[1:]); dbPath != "" {
		cfg.DBPath = dbPath
		os.Args = append([]string{os.Args[0]}, rest...)
	}

	svc, _, store, err := app.Build(cfg)
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

// extractDBFlag returns the --db flag value and the remaining args (with the
// flag and its value removed), supporting both "--db path" and "--db=path".
// It is applied before cobra parses so the store can be opened at the resolved
// path; the flag itself is re-registered on the root command for --help.
func extractDBFlag(args []string) (string, []string) {
	dbPath := ""
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--db":
			if i+1 < len(args) {
				dbPath = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--db="):
			dbPath = strings.TrimPrefix(arg, "--db=")
		default:
			rest = append(rest, arg)
		}
	}
	return dbPath, rest
}
