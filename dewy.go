package dewy

import (
	"github.com/lestrrat-go/server-starter"
	"github.com/linyows/dewy/kvs"
)

type Dewy struct {
	config     Config
	repository Repository
	cache      kvs.KVS
	starter    starter.Config
}

func New(c Config) *Dewy {
	return &Dewy{
		config: c,
	}
}

func (d *Dewy) Run() error {
	// c := New("file", kvs.Config)
	// c.Read(d.config.Repository.String())
	d.config.Repository.String()
	r := NewRepository(d.config.Repository)
	if err := r.Fetch(); err != nil {
		return err
	}
	r.Download()
	return nil
}
