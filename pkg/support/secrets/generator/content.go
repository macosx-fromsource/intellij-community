package generator

type Content map[string][]byte

func (c Content) AsMap() map[string][]byte {
	return map[string][]byte(c)
}

func (c Content) Get(key string) []byte {
	return c[key]
}

func (c Content) GetString(key string) string {
	return string(c[key])
}
