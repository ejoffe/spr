package github

import (
	"fmt"
	"unicode/utf8"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/terminal"
)

type PullRequest struct {
	ID         string
	Number     int
	FromBranch string
	ToBranch   string
	Commit     git.Commit
	Title      string

	MergeStatus PullRequestMergeStatus
	Merged      bool
}

type checkStatus int

const (
	CheckStatusUnknown checkStatus = iota
	CheckStatusPending
	CheckStatusPass
	CheckStatusFail
)

type PullRequestMergeStatus struct {
	ChecksPass     checkStatus
	ReviewApproved bool
	NoConflicts    bool
	Stacked        bool
}

// SortPullRequests sorts the pull requests so that the one that is on top of
//  master will come first followed by the ones that are stacked on top.
// The stack order is maintained so that multiple pull requests can be merged in
//  the correct order.
func SortPullRequests(prs []*PullRequest, config *config.Config) []*PullRequest {

	swap := func(i int, j int) {
		buf := prs[i]
		prs[i] = prs[j]
		prs[j] = buf
	}

	targetBranch := "master"
	j := 0
	for i := 0; i < len(prs); i++ {
		for j = i; j < len(prs); j++ {
			if prs[j].ToBranch == targetBranch {
				targetBranch = prs[j].FromBranch
				swap(i, j)
				break
			}
		}
	}

	// update stacked merge status flag
	for _, pr := range prs {
		if pr.Ready(config) {
			pr.MergeStatus.Stacked = true
		} else {
			break
		}
	}

	return prs
}

func (pr *PullRequest) Mergeable(config *config.Config) bool {
	if !pr.MergeStatus.NoConflicts {
		return false
	}
	if !pr.MergeStatus.Stacked {
		return false
	}
	if config.RequireChecks && pr.MergeStatus.ChecksPass != CheckStatusPass {
		return false
	}
	if config.RequireApproval && !pr.MergeStatus.ReviewApproved {
		return false
	}
	return true
}

func (pr *PullRequest) Ready(config *config.Config) bool {
	if pr.Commit.WIP {
		return false
	}
	if !pr.MergeStatus.NoConflicts {
		return false
	}
	if config.RequireChecks && pr.MergeStatus.ChecksPass != CheckStatusPass {
		return false
	}
	if config.RequireApproval && !pr.MergeStatus.ReviewApproved {
		return false
	}
	return true
}

const checkmark = "\xE2\x9C\x94"
const crossmark = "\xE2\x9C\x97"
const middledot = "\xC2\xB7"

func (pr *PullRequest) StatusString(config *config.Config) string {
	statusString := "["

	statusString += pr.MergeStatus.ChecksPass.String(config)

	if config.RequireApproval {
		if pr.MergeStatus.ReviewApproved {
			statusString += checkmark
		} else {
			statusString += crossmark
		}
	} else {
		statusString += "-"
	}

	if pr.MergeStatus.NoConflicts {
		statusString += checkmark
	} else {
		statusString += crossmark
	}

	if pr.MergeStatus.Stacked {
		statusString += checkmark
	} else {
		statusString += crossmark
	}

	statusString += "]"
	return statusString
}

func (pr *PullRequest) String(config *config.Config) string {
	prStatus := pr.StatusString(config)
	if pr.Merged {
		prStatus = "MERGED"
	}

	prInfo := fmt.Sprintf("%3d", pr.Number)
	if config.ShowPRLink {
		prInfo = fmt.Sprintf("github.com/%s/%s/pull/%d",
			config.GitHubRepoOwner, config.GitHubRepoName, pr.Number)
	}

	line := fmt.Sprintf("%s %s : %s", prStatus, prInfo, pr.Title)

	// trim line to terminal width
	terminalWidth, err := terminal.Width()
	if err != nil {
		terminalWidth = 1000
	}
	lineByteLength := len(line)
	lineLength := utf8.RuneCountInString(line)
	diff := lineLength - terminalWidth
	if diff > 0 {
		line = line[:lineByteLength-diff-3] + "..."
	}

	return line
}

func (cs checkStatus) String(config *config.Config) string {
	if config.RequireChecks {
		switch cs {
		case CheckStatusUnknown:
			return "?"
		case CheckStatusPending:
			return middledot
		case CheckStatusFail:
			return crossmark
		case CheckStatusPass:
			return checkmark
		default:
			return "?"
		}
	}
	return "-"
}
