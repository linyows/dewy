package grpc

type Config struct {
	Target string `schema:"-"`
	NoTLS  bool   `schema:"no-tls"`
}
