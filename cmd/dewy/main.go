package main

import (
	"runtime"

	"github.com/carlescere/scheduler"
	"github.com/linyows/dewy"
)

const (
	// ExitOK for exit code
	ExitOK int = 0

	// ExitErr for exit code
	ExitErr int = 1
)

type CLI struct {
	outStream, errStream io.Writer
	Command              string
	Config               string `long:"config" short:"c" description:"Path to configuration file"`
	LogLevel             string `long:"log-level" short:"l" arg:"(debug|info|warn|error)" description:"Level displayed as log"`
	Interval             string `long:"interval" short:"i" description:"The polling interval to the repository"`
	Help                 bool   `long:"help" short:"h" description:"show this help message and exit"`
	Version              bool   `long:"version" short:"v" description:"prints the version number"`
}

func main() {
	job := func() {
		c := dewy.Config{
			Cache: dewy.CacheConfig{
				Type:       dewy.FILE,
				Expiration: 10,
			},
			Repository: dewy.RepositoryConfig{
				Name:     "mox",
				Owner:    "linyows",
				Artifact: "darwin_amd64.zip",
			},
		}
		c.OverrideWithEnv()
		d := dewy.New(c)
		if err := d.Run(); err != nil {
			panic(err)
		}
	}
	scheduler.Every(10).Seconds().NotImmediately().Run(job)
	runtime.Goexit()
}
