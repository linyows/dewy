package kvs

// Redis struct
type Redis struct {
	items    map[string]*item //nolint
	Host     string
	Port     int
	Password string
	TTL      int
}

// Read data by key on redis
func (r *Redis) Read(key string) {
}

// Write data to redis
func (r *Redis) Write(data string) {
}

// Delete key on redis
func (r *Redis) Delete(key string) {
}

// List returns keys from redis
func (r *Redis) List() {
}
