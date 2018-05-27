package kvs

type Consul struct {
	items    map[string]*item
	Host     string
	Port     int
	Password string
}

func (c *Consul) Read(key string) {
}

func (c *Consul) Write(data string) {
}

func (c *Consul) Delete(key string) {
}

func (c *Consul) List() {
}
