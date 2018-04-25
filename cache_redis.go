package dewy

type RedisCache struct {
	ID       string
	Data     Data
	Host     string
	Port     int
	Password string
	TTL      int
}

func (r *RedisCache) Read(key string) {
}

func (r *RedisCache) Write(data string) {
}

func (r *RedisCache) Delete(key string) {
}

func (r *RedisCache) List() {
}
