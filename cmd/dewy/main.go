package main

import (
	"runtime"

	"github.com/carlescere/scheduler"
	"github.com/linyows/dewy"
)

func main() {
	job := func() {
		c := dewy.Config{
			Cache: dewy.CacheConfig{
				Type:       dewy.FILE,
				Expiration: 10,
			},
			Repository: dewy.RepositoryConfig{
				Name:  "octopass",
				Owner: "linyows",
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
