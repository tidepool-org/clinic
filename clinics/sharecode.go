package clinics

import (
	"math/rand"
	"strings"
)

const (
	shareCodeGroupLength = 4
	shareCodeGroupCount  = 3
	separator            = "-"
	characters           = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

type ShareCodeGenerator interface {
	Generate() string
}

func NewShareCodeGenerator() (ShareCodeGenerator, error) {
	return &shareCodeGenerator{
		groupCount:  shareCodeGroupCount,
		groupLength: shareCodeGroupLength,
		separator:   separator,
		chars:       characters,
	}, nil
}

type shareCodeGenerator struct {
	groupCount  int
	groupLength int
	separator   string
	chars       string
}

func (s *shareCodeGenerator) Generate() string {
	groups := make([]string, s.groupCount)
	for i, _ := range groups {
		groups[i] = generateRandomStringFromAlphabet(s.chars, s.groupLength)
	}
	return strings.Join(groups, s.separator)
}

func generateRandomStringFromAlphabet(chars string, length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
