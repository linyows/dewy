package main

import (
	"os"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli := &CLI{outStream: os.Stdout, errStream: os.Stderr, Interval: -1}
	cli.run(os.Args[1:])
}
