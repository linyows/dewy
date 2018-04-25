package dewy

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
