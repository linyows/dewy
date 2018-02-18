package dewy

import (
	"time"

	starter "github.com/lestrrat-go/server-starter"
)

type Config struct {
	Repository          string
	Interval            time.Duration
	ServerStarterConfig starter.Config
}
