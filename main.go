package main

import (
	"fmt"
	"os"

	"wslc-compose/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "wslc-compose: "+err.Error())
		os.Exit(1)
	}
}
