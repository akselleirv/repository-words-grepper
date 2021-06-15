package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
)

const (
	ConfigFilePath = "./config.json"
	ResultFilePath = "./results.json"

	GrepErrorCodeNoMatches = 1
)

type Config struct {
	SearchWords  []string     `json:"search_words"`
	ExcludeDirs  []string     `json:"exclude_dirs"`
	Repositories []Repository `json:"repositories"`
}
type Repository struct {
	Name        string   `json:"name"`
	Url         string   `json:"url"`
	ExcludeDirs []string `json:"exclude_dirs"`
}
type ResultFile struct {
	TotalApplications int           `json:"total_applications"`
	SearchWords       []string      `json:"search_words"`
	TotalCountSum     int           `json:"total_count_sum"`
	Applications      []Application `json:"applications"`
}
type Application struct {
	Name        string       `json:"name"`
	CountSum    int          `json:"count_sum"`
	GrepResults []GrepResult `json:"grep_results"`
}
type GrepResult struct {
	FileName string `json:"file_name"`
	Count    int    `json:"count"`
}

func main() {
	var results ResultFile
	cfg, err := loadConfig(ConfigFilePath)
	if err != nil {
		log.Fatal(err)
	}

	results.TotalApplications = len(cfg.Repositories)
	results.Applications = make([]Application, results.TotalApplications)
	results.SearchWords = cfg.SearchWords

	var wg sync.WaitGroup
	wg.Add(results.TotalApplications)
	for i, repo := range cfg.Repositories {
		go func(repo Repository, index int) {
			defer wg.Done()
			result, err := analyzeRepo(repo, cfg.SearchWords, append(cfg.ExcludeDirs, cfg.Repositories[index].ExcludeDirs...))
			if err != nil {
				log.Fatalf("failed on repo '%s': %s", repo.Name, err.Error())
			}
			results.Applications[index] = Application{
				Name:        repo.Name,
				CountSum:    sumTotalCountForGrepResults(result),
				GrepResults: result,
			}
		}(repo, i)

	}

	wg.Wait()
	results.TotalCountSum = calculateTotalCountSum(results)
	if err := writeResult(ResultFilePath, sortOnAppCountSumDesc(results)); err != nil {
		log.Fatal("unable to save result: %w", err)
	}
}

func sortOnAppCountSumDesc(result ResultFile) ResultFile {
	sort.Slice(result.Applications, func(i, j int) bool {
		return result.Applications[i].CountSum > result.Applications[j].CountSum
	})
	return result
}

func sumTotalCountForGrepResults(grs []GrepResult) int {
	var result int
	for _, gr := range grs {
		result += gr.Count
	}
	return result
}

func analyzeRepo(r Repository, searchWords, excludeDirs []string) ([]GrepResult, error) {
	path, removeDir, err := cloneRepo(r)
	if err != nil || removeDir == nil {
		return nil, err
	}
	defer removeDir()

	result, err := grep(path, searchWords, excludeDirs)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func calculateTotalCountSum(rf ResultFile) int {
	var result int
	for _, app := range rf.Applications {
		result += app.CountSum
	}
	return result
}

// loadConfig gets the repos information from the given filename
func loadConfig(filename string) (Config, error) {
	var cfg Config
	file, err := os.ReadFile(filename)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(file, &cfg)
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

// grep uses the grep command in OS and searches for the given searchWords
func grep(path string, searchWords, excludeDirs []string) ([]GrepResult, error) {
	args := grepExcludeDirStr(excludeDirs)
	args = append(args, searchWordsStr(searchWords)...)
	args = append(args, "--recursive", "--ignore-case", "--only-matching", path)

	grepCmd := exec.Command("grep", args...)
	log.Println("running command: " + strings.Join(grepCmd.Args, " "))
	grepOut, err := grepCmd.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			if exitError.ExitCode() == GrepErrorCodeNoMatches {
				return []GrepResult{}, nil
			}
			return nil, fmt.Errorf("unable to execute grep command: %s", string(exitError.Stderr))
		}
		return nil, fmt.Errorf("unable to execute grep command: %w", err)
	}
	return parseGrepOutput(string(grepOut), path), nil
}

func searchWordsStr(searchWords []string) []string {
	var result []string
	for _, word := range searchWords {
		result = append(result, "--regexp="+word)
	}
	return result
}

func grepExcludeDirStr(excludeDirs []string) []string {
	var result []string
	for _, dir := range excludeDirs {
		result = append(result, "--exclude-dir="+dir)
	}
	return result
}

func parseGrepOutput(out, basePath string) []GrepResult {
	var results []GrepResult
	pathCounts := make(map[string]int)

	for _, line := range strings.Split(out, "\n") {
		if path, searchWord := splitOutputLine(line); path != "" && searchWord != "" {
			pathCounts[removeBasePath(path, basePath)] += 1
		}
	}

	for path, count := range pathCounts {
		results = append(results, GrepResult{
			FileName: path,
			Count:    count,
		})
	}

	return results
}

// splitOutputLine splits the output: <path>:<search-word>
func splitOutputLine(grepLine string) (string, string) {
	split := strings.Split(grepLine, ":")
	if len(split) == 2 {
		return split[0], split[1]
	}

	return "", ""
}

func removeBasePath(path, basePath string) string {
	if cleanName := strings.Split(path, basePath+"/"); len(cleanName) >= 1 {
		return cleanName[1]
	}
	return ""
}

func writeResult(fileName string, data ResultFile) error {
	file, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		return err
	}
	if err = ioutil.WriteFile(fileName, file, 0664); err != nil {
		return err
	}
	return nil
}

type removeDir = func()

// cloneRepo clones the given repo using 'git clone' and returns the path to the cloned repo and a func to remove it in the filesystem
func cloneRepo(r Repository) (string, removeDir, error) {
	dir, err := ioutil.TempDir("", "clone")
	if err != nil {
		return "", nil, err
	}
	removeDir := func() {
		func(path string) {
			err := os.RemoveAll(path)
			if err != nil {
				log.Println("unable to remove dir: ", err)
			}
		}(dir)
	}

	cloneCmd := exec.Command("git", "clone", r.Url, dir)
	log.Println("running command: " + strings.Join(cloneCmd.Args, " "))
	if err := cloneCmd.Run(); err != nil {
		removeDir()
		log.Fatal("unable to git clone "+r.Name, err)
	}

	return dir, removeDir, nil
}
