package generator

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
)

type Size uint32

var ErrInvalidSize = errors.New("invalid size")
var validSizes = []uint64{0, 256, 384, 521, 1024, 2048, 4096}

func ParseSize(annotation string) (Size, error) {
	size, err := strconv.ParseUint(annotation, 10, 64)

	if numError, ok := err.(*strconv.NumError); ok {
		err = numError.Err
	}

	if err != nil {
		return Size(0), err
	}

	if !slices.Contains(validSizes, size) {
		return Size(0), fmt.Errorf("size: %d %w", size, ErrInvalidSize)
	}

	return Size(size), nil
}

func (s Size) Uint64() uint64 {
	return uint64(s)
}

func (s Size) String() string {
	return strconv.FormatUint(uint64(s), 10)
}
