package spr

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

// Config object to hold spr configuration
type Config struct {
	GitHubRepoOwner string `yaml:"githubRepoOwner"`
	GitHubRepoName  string `yaml:"githubRepoName"`

	RequireChecks   bool `yaml:"requireChecks"`
	RequireApproval bool `yaml:"requireApproval"`
}

// ReadConfig looks for a .spr.yml file in the root git directory.
//  if found, the config is read and returned.
//  if not found, a default config is created written to the config file and
//   returned.
func ReadConfig() *Config {
	var rootdir string
	mustgit("rev-parse --show-toplevel", &rootdir)
	rootdir = strings.TrimSpace(rootdir)
	filename := rootdir + "/.spr.yml"
	config := readConfigFile(filename)
	return config
}

func readConfigFile(filename string) *Config {
	var config *Config

	configfile, err := os.Open(filepath.Clean(filename))
	if err != nil {
		if os.IsNotExist(err) {
			config = defaultConfig()
			writeConfigFile(filename, config)
			installCommitHook()
			return config
		} else {
			check(err)
		}
	} else {
		decoder := yaml.NewDecoder(configfile)
		err = decoder.Decode(&config)
		check(err)
	}
	return config
}

func defaultConfig() *Config {
	var output string
	mustgit("remote -v", &output)
	lines := strings.Split(output, "\n")

	regex := regexp.MustCompile(`^origin\s+https://github.com/(\w+)/(\w+).git \(push\)`)

	for _, line := range lines {
		matches := regex.FindStringSubmatch(line)
		if matches != nil {
			return &Config{
				GitHubRepoOwner: matches[1],
				GitHubRepoName:  matches[2],
				RequireChecks:   true,
				RequireApproval: true,
			}
		}
	}
	return &Config{
		RequireChecks:   true,
		RequireApproval: true,
	}
}

func writeConfigFile(filename string, config *Config) {
	configfile, err := os.Create(filepath.Clean(filename))
	check(err)
	encoder := yaml.NewEncoder(configfile)
	err = encoder.Encode(config)
	check(err)
	configfile.Close()
	fmt.Printf("- Config file not found.\n")
	fmt.Printf("- Default config file created %s\n", filename)
}

func installCommitHook() {
	var rootdir string
	mustgit("rev-parse --show-toplevel", &rootdir)
	rootdir = strings.TrimSpace(rootdir)
	err := os.Chdir(rootdir)
	check(err)
	path, err := exec.LookPath("spr_commit_hook")
	check(err)
	cmd := exec.Command("ln", "-s", path, ".git/hooks/commit-msg")
	_, err = cmd.CombinedOutput()
	check(err)
	fmt.Printf("- Installed commit hook in .git/hooks/commit-msg\n")
}
