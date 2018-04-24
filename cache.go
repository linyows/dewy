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
