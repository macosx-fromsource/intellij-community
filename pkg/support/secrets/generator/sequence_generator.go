package generator

import (
	"crypto/rand"
	"fmt"
)

const randomReadOverhead = 4

type SequenceGenerator struct {
	characterSets []CharacterSet
	length        Length
}

func NewSequenceGenerator(lengthAnnotation string, characterSetAnnotations []string) (*SequenceGenerator, error) {
	length, err := ParseLength(lengthAnnotation)
	if err != nil {
		return nil, err
	}

	characterSets := make([]CharacterSet, len(characterSetAnnotations))
	for i, cs := range characterSetAnnotations {
		characterSets[i], err = ParseCharacterSet(cs)
		if err != nil {
			return nil, err
		}
	}

	g := SequenceGenerator{
		characterSets: characterSets,
		length:        length,
	}

	return &g, nil
}

func (g SequenceGenerator) allowedCharacters() string {
	chars := ""

	for _, cs := range g.characterSets {
		chars = chars + cs.String()
	}

	return chars
}

func (g SequenceGenerator) Generate(contentKey string) (Content, error) {
	allowedCharacters := g.allowedCharacters()

	allowedLen := len(allowedCharacters)
	if allowedLen == 0 || allowedLen > 255 {
		return nil, fmt.Errorf("allowed character set length %d is out of range (1-255)", allowedLen)
	}

	var (
		seq    = make([]byte, g.length)
		mod    = byte(allowedLen)
		reject = 255 - (255 % mod)
		bytes  = make([]byte, g.length*randomReadOverhead)
	)

	var i uint64 = 0
	for i < g.length.Uint64() {
		if _, err := rand.Read(bytes); err != nil {
			return nil, err
		}

		for _, b := range bytes {
			if i >= g.length.Uint64() {
				break
			}

			if b < reject {
				seq[i] = allowedCharacters[b%mod]
				i++
			}
		}
	}

	return Content{contentKey: seq}, nil
}
