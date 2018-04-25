package dewy

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
