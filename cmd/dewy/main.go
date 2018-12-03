package main

import (
	"os"

	"github.com/linyows/dewy"
)

func main() {
	os.Exit(dewy.RunCLI(os.Stdout, os.Stderr, os.Args[1:]))
}
