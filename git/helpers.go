package git

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/ejoffe/spr/config"
	"github.com/rs/zerolog/log"
)

func GetLocalBranchName(gitcmd GitInterface) string {
	var output string
	err := gitcmd.Git("branch --no-color", &output)
	check(err)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "* ") {
			return line[2:]
		}
	}
	panic("cannot determine local git branch name")
}

func GetRemoteBranchName(repoConfig *config.RepoConfig, gitcmd GitInterface) string {
	localBranchName := GetLocalBranchName(gitcmd)

	for _, remoteBranchName := range repoConfig.RemoteBranches {
		if localBranchName == remoteBranchName {
			return remoteBranchName
		}
	}
	return repoConfig.GitHubBranch
}

// getLocalCommitStack returns a list of unmerged commits
func GetLocalCommitStack(repoConfig *config.RepoConfig, gitcmd GitInterface) []Commit {
	var commitLog string
	targetBranch := GetRemoteBranchName(repoConfig, gitcmd)
	logCommand := fmt.Sprintf("log --format=medium --no-color %s/%s..HEAD",
		repoConfig.GitHubRemote, targetBranch)
	gitcmd.MustGit(logCommand, &commitLog)
	commits, valid := parseLocalCommitStack(commitLog)
	if !valid {
		// if not valid - run rebase to add commit ids
		rewordPath, err := exec.LookPath("spr_reword_helper")
		check(err)
		rebaseCommand := fmt.Sprintf("rebase %s/%s -i --autosquash --autostash",
			repoConfig.GitHubRemote, targetBranch)
		gitcmd.GitWithEditor(rebaseCommand, nil, rewordPath)

		gitcmd.MustGit(logCommand, &commitLog)
		commits, valid = parseLocalCommitStack(commitLog)
		if !valid {
			// if still not valid - panic
			errMsg := "unable to fetch local commits\n"
			errMsg += " most likely this is an issue with missing commit-id in the commit body\n"
			panic(errMsg)
		}
	}
	return commits
}

func parseLocalCommitStack(commitLog string) ([]Commit, bool) {
	var commits []Commit

	commitHashRegex := regexp.MustCompile(`^commit ([a-f0-9]{40})`)
	commitIDRegex := regexp.MustCompile(`commit-id\:([a-f0-9]{8})`)

	// The list of commits from the command line actually starts at the
	//  most recent commit. In order to reverse the list we use a
	//  custom prepend function instead of append
	prepend := func(l []Commit, c Commit) []Commit {
		l = append(l, Commit{})
		copy(l[1:], l)
		l[0] = c
		return l
	}

	// commitScanOn is set to true when the commit hash is matched
	//  and turns false when the commit-id is matched.
	//  Commit messages always start with a hash and end with a commit-id.
	//  The commit subject and body are always between the hash the commit-id.
	commitScanOn := false

	subjectIndex := 0
	var scannedCommit Commit

	lines := strings.Split(commitLog, "\n")
	log.Debug().Int("lines", len(lines)).Msg("parseLocalCommitStack")
	for index, line := range lines {

		// match commit hash : start of a new commit
		matches := commitHashRegex.FindStringSubmatch(line)
		if matches != nil {
			log.Debug().Interface("matches", matches).Msg("parseLocalCommitStack :: commitHashMatch")
			if commitScanOn {
				// missing the commit-id
				log.Debug().Msg("parseLocalCommitStack :: missing commit id")
				return nil, false
			}
			commitScanOn = true
			scannedCommit = Commit{
				CommitHash: matches[1],
			}
			subjectIndex = index + 4
		}

		// match commit id : last thing in the commit
		matches = commitIDRegex.FindStringSubmatch(line)
		if matches != nil {
			log.Debug().Interface("matches", matches).Msg("parseLocalCommitStack :: commitIdMatch")
			scannedCommit.CommitID = matches[1]
			scannedCommit.Body = strings.TrimSpace(scannedCommit.Body)

			if strings.HasPrefix(scannedCommit.Subject, "WIP") {
				scannedCommit.WIP = true
			}

			commits = prepend(commits, scannedCommit)
			commitScanOn = false
		}

		// look for subject and body
		if commitScanOn {
			if index == subjectIndex {
				scannedCommit.Subject = strings.TrimSpace(line)
			} else if index == (subjectIndex+1) && line != "\n" {
				scannedCommit.Body += strings.TrimSpace(line) + "\n"
			} else if index > (subjectIndex + 1) {
				scannedCommit.Body += strings.TrimSpace(line) + "\n"
			}
		}

	}

	// if commitScanOn is true here it means there was a commit without
	//  a commit-id
	if commitScanOn {
		// missing the commit-id
		log.Debug().Msg("parseLocalCommitStack :: missing last commit id")
		return nil, false
	}

	log.Debug().Interface("commits", commits).Msg("parseLocalCommitStack")
	return commits, true
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
