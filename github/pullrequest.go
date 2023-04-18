package github

import (
	"fmt"
	"unicode/utf8"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/terminal"
)

// PullRequest has GitHub pull request data
type PullRequest struct {
	ID         string
	Number     int
	FromBranch string
	ToBranch   string
	Commit     git.Commit
	Title      string
	Body       string

	MergeStatus PullRequestMergeStatus
	Merged      bool
}

type checkStatus int

const (
	// CheckStatusUnknown
	CheckStatusUnknown checkStatus = iota

	// CheckStatusPending when checks are still running
	CheckStatusPending

	// CheckStatusPass when all checks pass
	CheckStatusPass

	// CheckStatusFail when some chechs have failed
	CheckStatusFail
)

// PullRequestMergeStatus is the merge status of a pull request
type PullRequestMergeStatus struct {
	// ChecksPass is the status of GitHub checks
	ChecksPass checkStatus

	// ReviewApproved is true when a pull request is approved by a fellow reviewer
	ReviewApproved bool

	// NoConflicts is true when there are no merge conflicts
	NoConflicts bool

	// Stacked is true when all requests in the stack up to this one are ready to merge
	Stacked bool
}

// Mergeable returns true if the pull request is mergable
func (pr *PullRequest) Mergeable(config *config.Config) bool {
	if !pr.MergeStatus.NoConflicts {
		return false
	}
	if !pr.MergeStatus.Stacked {
		return false
	}
	if config.Repo.RequireChecks && pr.MergeStatus.ChecksPass != CheckStatusPass {
		return false
	}
	if config.Repo.RequireApproval && !pr.MergeStatus.ReviewApproved {
		return false
	}
	return true
}

// Ready returns true if pull request is ready to merge
func (pr *PullRequest) Ready(config *config.Config) bool {
	if pr.Commit.WIP {
		return false
	}
	if !pr.MergeStatus.NoConflicts {
		return false
	}
	if config.Repo.RequireChecks && pr.MergeStatus.ChecksPass != CheckStatusPass {
		return false
	}
	if config.Repo.RequireApproval && !pr.MergeStatus.ReviewApproved {
		return false
	}
	return true
}

const (
	// Terminal escape codes for colors
	colorReset = "\033[0m"
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorBlue  = "\033[34m"

	// ascii status bits
	asciiCheckmark = "✔"
	asciiCrossmark = "✗"
	asciiPending   = "·"
	asciiQuerymark = "?"
	asciiEmpty     = "-"

	// emoji status bits
	emojiCheckmark    = "✅"
	emojiCrossmark    = "❌"
	emojiPending      = "⌛"
	emojiQuestionmark = "❓"
	emojiEmpty        = "➖"
)

func statusBitIcons(config *config.Config) map[string]string {
	if config.User.StatusBitsEmojis {
		return map[string]string{
			"checkmark":    emojiCheckmark,
			"crossmark":    emojiCrossmark,
			"pending":      emojiPending,
			"questionmark": emojiQuestionmark,
			"empty":        emojiEmpty,
		}
	} else {
		return map[string]string{
			"checkmark":    asciiCheckmark,
			"crossmark":    asciiCrossmark,
			"pending":      asciiPending,
			"questionmark": asciiQuerymark,
			"empty":        asciiEmpty,
		}
	}
}

// StatusString returs a string representation of the merge status bits
func (pr *PullRequest) StatusString(config *config.Config) string {
	icons := statusBitIcons(config)
	statusString := "["

	statusString += pr.MergeStatus.ChecksPass.String(config)

	if config.Repo.RequireApproval {
		if pr.MergeStatus.ReviewApproved {
			statusString += icons["checkmark"]
		} else {
			statusString += icons["crossmark"]
		}
	} else {
		statusString += icons["empty"]
	}

	if pr.MergeStatus.NoConflicts {
		statusString += icons["checkmark"]
	} else {
		statusString += icons["crossmark"]
	}

	if pr.MergeStatus.Stacked {
		statusString += icons["checkmark"]
	} else {
		statusString += icons["crossmark"]
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
	if config.User.ShowPRLink {
		prInfo = fmt.Sprintf("https://%s/%s/%s/pull/%d",
			config.Repo.GitHubHost, config.Repo.GitHubRepoOwner, config.Repo.GitHubRepoName, pr.Number)
	}

	line := fmt.Sprintf("%s %s : %s", prStatus, prInfo, pr.Title)

	// trim line to terminal width
	terminalWidth, err := terminal.Width()
	if err != nil {
		terminalWidth = 1000
	}
	lineLength := utf8.RuneCountInString(line)
	if config.User.StatusBitsEmojis {
		// each emoji consumes 2 chars in the terminal
		lineLength += 4
	}
	diff := lineLength - terminalWidth
	if diff > 0 && terminalWidth > 3 {
		line = line[:terminalWidth-3] + "..."
	}

	return line
}

func (cs checkStatus) String(config *config.Config) string {
	icons := statusBitIcons(config)
	if config.Repo.RequireChecks {
		switch cs {
		case CheckStatusUnknown:
			return icons["questionmark"]
		case CheckStatusPending:
			return icons["pending"]
		case CheckStatusFail:
			return icons["crossmark"]
		case CheckStatusPass:
			return icons["checkmark"]
		default:
			return icons["questionmark"]
		}
	}
	return icons["empty"]
}
