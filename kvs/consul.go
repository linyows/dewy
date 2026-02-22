package kvs

// Consul struct.
type Consul struct {
	items    map[string]*item //nolint
	Host     string
	Port     int
	Password string //nolint:gosec // G117
}

// Read data on Consul.
func (c *Consul) Read(key string) {
}

// Write data to Consul.
func (c *Consul) Write(data string) {
}

// Delete data on Consul.
func (c *Consul) Delete(key string) {
}

// List returns key from Consul.
func (c *Consul) List() {
}
