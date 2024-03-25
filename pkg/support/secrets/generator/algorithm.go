package generator

import (
	"errors"
	"fmt"
)

type Algorithm string

const (
	RSA     = Algorithm("rsa")
	ED25519 = Algorithm("ed25519")
	ECDSA   = Algorithm("ecdsa")
)

var ErrInvalidAlgorithm = errors.New("invalid algorithm")

func ParseAlgorithm(annotation string) (Algorithm, error) {
	switch annotation {
	case RSA.String():
		return RSA, nil
	case ED25519.String():
		return ED25519, nil
	case ECDSA.String():
		return ECDSA, nil
	default:
		return Algorithm(""), fmt.Errorf("algorithm: %s %w", annotation, ErrInvalidAlgorithm)
	}
}

func (a Algorithm) String() string {
	return string(a)
}
