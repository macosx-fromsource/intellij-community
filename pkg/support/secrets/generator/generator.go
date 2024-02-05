package generator

type Generator interface {
	Generate(key string) (Content, error)
}
