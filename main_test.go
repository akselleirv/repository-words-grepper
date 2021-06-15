package main

import (
	is2 "github.com/matryer/is"
	"strings"
	"testing"
)

func TestParseGrepOutput(t *testing.T) {
	is := is2.New(t)
	expectedNames := []string{"config.json", "go.mod", "results.json", "test.txt"}
	testInput := []string{
		"repository-words-grepper/config.json:fell",
		"repository-words-grepper/go.mod:fell",
		"repository-words-grepper/results.json:fell",
		"repository-words-grepper/results.json:fell",
		"repository-words-grepper/results.json:FELL",
		"repository-words-grepper/test.txt:FELL",
	}

	parsed := parseGrepOutput(strings.Join(testInput, "\n"), "repository-words-grepper")
	
	is.Equal(len(expectedNames), len(parsed))
	for i, result := range parsed {
		is.Equal(expectedNames[i], result.FileName)
	}
}
