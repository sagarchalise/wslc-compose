package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"wslc-compose/internal/compose"
	"wslc-compose/internal/wslc"
)

func Run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: wslc-compose <up|down|start|stop> [-f compose-file] [-p project-name]")
	}
	sub := args[0]
	rest := args[1:]

	fs := flag.NewFlagSet(sub, flag.ContinueOnError)
	file := fs.String("f", "", "path to compose file (default: compose.yaml/compose.yml/docker-compose.yaml/docker-compose.yml in current directory)")
	project := fs.String("p", "", "project name override")
	if err := fs.Parse(rest); err != nil {
		return err
	}

	switch sub {
	case "up", "down", "start", "stop":
	default:
		return fmt.Errorf("unknown command %q (expected up, down, start, or stop)", sub)
	}

	composePath, err := resolveComposeFile(*file)
	if err != nil {
		return err
	}

	proj, warnings, err := compose.Load(composePath, *project)
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warning: "+w)
	}
	if err != nil {
		return err
	}

	client := wslc.NewClient()

	switch sub {
	case "up":
		return Up(client, proj)
	case "down":
		return Down(client, proj)
	case "start":
		return Start(client, proj)
	case "stop":
		return Stop(client, proj)
	}
	return nil
}

func resolveComposeFile(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("compose file %q not found", explicit)
		}
		return filepath.Abs(explicit)
	}
	candidates := []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return filepath.Abs(c)
		}
	}
	return "", fmt.Errorf("no compose file found (looked for %v); use -f to specify one", candidates)
}
