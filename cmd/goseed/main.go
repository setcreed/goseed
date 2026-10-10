package main

import (
	"fmt"
	"os"

	"github.com/setcreed/goseed/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.NewCommand(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
