package dewy

type Cache interface {
	Read(key string) string
	Write(data string) bool
	Delete(key string) bool
	List() []string
}

func NewCache(c CacheConfig) Cache {
	switch c.Type {
	case FILE:
		return &FileCache{}
	default:
		panic("no cache provider")
	}
}

type Data struct {
	name string
	size uint
}

type MemoryCache struct {
	ID   string
	Data Data
}

func (m *MemoryCache) Read(key string) string {
	return ""
}

func (m *MemoryCache) Write(data string) bool {
	return true
}

func (m *MemoryCache) Delete(key string) bool {
	return true
}

func (m *MemoryCache) List() {
}

type FileCache struct {
	ID   string
	Data Data
	Path string
}

func (f *FileCache) Read(key string) string {
	return ""
}

func (f *FileCache) Write(data string) bool {
	return true
}

func (f *FileCache) Delete(key string) bool {
	return true
}

func (f *FileCache) List() []string {
	return []string{""}
}

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

type ConsulCache struct {
	ID       string
	Data     Data
	Host     string
	Port     int
	Password string
}

func (c *ConsulCache) Read(key string) {
}

func (c *ConsulCache) Write(data string) {
}

func (c *ConsulCache) Delete(key string) {
}

func (c *ConsulCache) List() {
}
