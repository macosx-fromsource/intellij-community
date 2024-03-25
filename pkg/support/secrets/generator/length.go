package generator

import (
	"errors"
	"fmt"
	"strconv"
)

type Length uint64

const maxSecretLength = 1048576

var ErrInvalidLength = errors.New("invalid length")

func ParseLength(annotation string) (Length, error) {
	length, err := strconv.ParseUint(annotation, 10, 64)

	if numError, ok := err.(*strconv.NumError); ok {
		err = numError.Err
	}

	if err != nil {
		return Length(0), err
	}

	if length == 0 || length > maxSecretLength {
		return Length(0), fmt.Errorf("length: %d %w", length, ErrInvalidLength)
	}

	return Length(length), nil
}

func (l Length) Uint64() uint64 {
	return uint64(l)
}

func (l Length) String() string {
	return strconv.FormatUint(uint64(l), 10)
}
