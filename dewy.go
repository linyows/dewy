package dewy

import (
	"github.com/lestrrat-go/server-starter"
)

type Dewy struct {
	config     Config
	repository Repository
	cache      Cache
	starter    starter.Config
}

func New(c Config) *Dewy {
	return &Dewy{
		config: c,
	}
}

func (d *Dewy) Run() error {
	r := NewRepository(d.config.Repository)
	if err := r.Fetch(); err != nil {
		return err
	}
	r.Download()
	return nil
}
