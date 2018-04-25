package dewy

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
