package mockgit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ejoffe/spr/git"
	"github.com/stretchr/testify/require"
)

func NewMockGit(t *testing.T) *mock {
	return &mock{
		assert: require.New(t),
	}
}

func (m *mock) Cmd(args string, output *string) error {
	fmt.Printf("CMD: git %s\n", args)

	expected := m.expectedCmd[0]
	actual := "git " + args
	m.assert.Equal(expected, actual)

	if m.response[0].valid {
		m.assert.NotNil(output)
		*output = generateResponse(m.response[0])
	} else {
		m.assert.Nil(output)
	}

	m.expectedCmd = m.expectedCmd[1:]
	m.response = m.response[1:]

	return nil
}

type mock struct {
	assert      *require.Assertions
	expectedCmd []string
	response    []cmdresponse
}

type cmdresponse struct {
	valid   bool
	commits []*git.Commit
}

func (m *mock) ExpectFetch() {
	m.expect("git fetch")
	m.expect("git rebase origin/master --autostash")
}

func (m *mock) ExpectLogAndRespond(commits []*git.Commit) {
	m.expect("git log origin/master..HEAD").respond(commits)
}

func (m *mock) ExpectPushCommits(commits []*git.Commit) {
	m.expect("git status --porcelain --untracked-files=no").respond(nil)

	for _, c := range commits {
		m.expect("git checkout %s", c.CommitHash)
		m.expect("git switch -C pr/TestSPR/master/%s", c.CommitID)
		m.expect("git push --force --set-upstream origin pr/TestSPR/master/%s", c.CommitID)
		m.expect("git switch master")
		m.expect("git branch -D pr/TestSPR/master/%s", c.CommitID)
	}
	m.expect("git switch master")
}

func (m *mock) expect(cmd string, args ...interface{}) *mock {
	m.expectedCmd = append(m.expectedCmd, fmt.Sprintf(cmd, args...))
	m.response = append(m.response, cmdresponse{valid: false})
	return m
}

func (m *mock) respond(commits []*git.Commit) {
	m.response[len(m.response)-1] = cmdresponse{
		valid:   true,
		commits: commits,
	}
}

func generateResponse(resp cmdresponse) string {
	if !resp.valid {
		return ""
	}

	var b strings.Builder
	for _, c := range resp.commits {
		fmt.Fprintf(&b, "commit %s\n", c.CommitHash)
		fmt.Fprintf(&b, "Author: Eitan Joffe <ejoffe@gmail.com>\n")
		fmt.Fprintf(&b, "Date:   Fri Jun 11 14:15:49 2021 -0700\n")
		fmt.Fprintf(&b, "\n")
		fmt.Fprintf(&b, "\t%s\n", c.Subject)
		fmt.Fprintf(&b, "\n")
		fmt.Fprintf(&b, "\tcommit-id:%s\n", c.CommitID)
		fmt.Fprintf(&b, "\n")
	}

	return b.String()
}
