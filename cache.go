package dewy

type Cache interface {
	Read()
	Write()
	IsExpired()
}

type Data struct {
	name string
	size uint
}

type MemoryCache struct {
	ID   string
	Data Data
}

func (m *MemoryCache) Read() {
}

func (m *MemoryCache) Write() {
}

func (m *MemoryCache) IsExpired() {
}

type FileCache struct {
	ID   string
	Data Data
	Path string
}

func (f *FileCache) Read() {
}

func (f *FileCache) Write() {
}

func (f *FileCache) IsExpired() {
}

type RedisCache struct {
	ID       string
	Data     Data
	Host     string
	Port     int
	Password string
	TTL      int
}

func (r *RedisCache) Read() {
}

func (r *RedisCache) Write() {
}

func (r *RedisCache) IsExpired() {
}

type ConsulCache struct {
	ID       string
	Data     Data
	Host     string
	Port     int
	Password string
}

func (c *ConsulCache) Read() {
}

func (c *ConsulCache) Write() {
}

func (c *ConsulCache) IsExpired() {
}
