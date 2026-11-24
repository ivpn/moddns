package idgen

import "errors"

const (
	TypeShortId      = "shortid"
	TypeRandom       = "random"
	TypeDeletionCode = "deletion-code"
)

var (
	ErrUnknownGen = errors.New("unknown generator type")
)

// Generator is an interface for generating unique IDs
type Generator interface {
	Generate() (string, error)
}

// NewGenerator creates a new Generator instance
func NewGenerator(genType string) (Generator, error) {
	switch genType {
	case TypeShortId:
		return NewShortIdGenerator()
	case TypeRandom:
		return NewRandomGenerator(), nil
	case TypeDeletionCode:
		return NewDeletionCodeGenerator()
	}
	return nil, ErrUnknownGen
}
