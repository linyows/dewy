package dewy

import (
	starter "github.com/lestrrat-go/server-starter"
)

type Config struct {
	Repository          string
	ServerStarterConfig starter.Config
}
