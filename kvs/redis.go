package kvs

type Redis struct {
	items    map[string]*item
	Host     string
	Port     int
	Password string
	TTL      int
}

func (r *Redis) Read(key string) {
}

func (r *Redis) Write(data string) {
}

func (r *Redis) Delete(key string) {
}

func (r *Redis) List() {
}
