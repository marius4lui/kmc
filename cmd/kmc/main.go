package main

import (
	"context"
	"fmt"
	"os"

	"github.com/marius4lui/kmc/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.Run(context.Background(), os.Args[1:], version); err != nil {
		fmt.Fprintf(os.Stderr, "kmc: %v\n", err)
		os.Exit(cli.ExitCode(err))
	}
}
