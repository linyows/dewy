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
