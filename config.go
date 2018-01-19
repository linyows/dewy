package dewy

import (
	"os"
	"time"
	"github.com/lestrrat/go-server-starter"
)

type Config interface {
	Repository string
	ServerStarterConfig starter.Config
}
