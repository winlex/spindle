package main

import (
	"fmt"
	"github.com/winlex/spindle/internal/buildinfo"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println(buildinfo.New().String())
	return nil
}
