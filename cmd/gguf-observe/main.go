package main

import (
	"fmt"
	"os"

	"github.com/Edge-Computing-LLM/gguf-observability/internal/observer"
)

func main() {
	if err := observer.RunCLI(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(observer.ExitCode(err))
	}
}
