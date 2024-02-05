package generator

import (
	"errors"
	"fmt"
)

type CharacterSet string

const (
	Lower  = CharacterSet("abcdefghijklmnopqrstuvwxyz")
	Upper  = CharacterSet("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	Digit  = CharacterSet("0123456789")
	Symbol = CharacterSet("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~")
)

var ErrInvalidCharacterSet = errors.New("invalid character set")

func ParseCharacterSet(annotation string) (CharacterSet, error) {
	switch annotation {
	case "lower":
		return Lower, nil
	case "upper":
		return Upper, nil
	case "digit":
		return Digit, nil
	case "symbol":
		return Symbol, nil
	default:
		return CharacterSet(""), fmt.Errorf("character set: %s %w", annotation, ErrInvalidCharacterSet)
	}
}

func (cs CharacterSet) String() string {
	return string(cs)
}
