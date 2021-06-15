package main

import (
	IS "github.com/matryer/is"
	"strings"
	"testing"
)

func TestParseGrepOutput(t *testing.T) {
	is := IS.New(t)
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

func TestGrep(t *testing.T) {
	is := IS.New(t)
	expectedResult := []GrepResult{
		{
			"testdata_1.txt",
			2,
		},
		{
			"testdata_2.txt",
			4,
		},
	}
	result, err := grep("./testdata", []string{"fell"}, []string{})

	is.NoErr(err)
	is.Equal(len(expectedResult), len(result))
	for i, r := range result {
		is.Equal(expectedResult[i].FileName, r.FileName)
		is.Equal(expectedResult[i].Count, r.Count)
	}

}
