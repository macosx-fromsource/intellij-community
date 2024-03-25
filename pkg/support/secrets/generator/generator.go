package generator

import "time"

var now = time.Now

type Generator interface {
	Generate(key string) (Content, error)
}
