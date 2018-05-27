package kvs

type Memory struct {
	items map[string]*item
}

func (m *Memory) Read(key string) string {
	return ""
}

func (m *Memory) Write(data string) bool {
	return true
}

func (m *Memory) Delete(key string) bool {
	return true
}

func (m *Memory) List() {
}
