package kvs

// Memory struct
type Memory struct {
	items map[string]*item
}

// Read data by key on memory
func (m *Memory) Read(key string) string {
	return ""
}

// Write data to memory
func (m *Memory) Write(data string) bool {
	return true
}

// Delete data by key on memory
func (m *Memory) Delete(key string) bool {
	return true
}

// List returns keys from memory
func (m *Memory) List() {
}
