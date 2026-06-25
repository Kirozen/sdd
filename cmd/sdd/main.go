package main

import (
	"fmt"
	"os"

	"github.com/kirozen/sdd/internal/sdd"
)

// version is injected at release build time via -X main.version (goreleaser).
var version = "0.0.1"

func main() {
	sdd.Version = version
	if err := sdd.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
