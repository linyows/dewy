package main

import (
	"os"

	"github.com/linyows/dewy"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(dewy.RunCLI(dewy.Env{
		Out:     os.Stdout,
		Err:     os.Stderr,
		Args:    os.Args[1:],
		Version: version,
		Commit:  commit,
		Date:    date,
	}))
}
