package mockgit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ejoffe/spr/git"
	"github.com/stretchr/testify/require"
)

// NewMockGit creates and new mock git instance
func NewMockGit(t *testing.T) *Mock {
	return &Mock{
		assert: require.New(t),
	}
}

func (m *Mock) GitWithEditor(args string, output *string, editorCmd string) error {
	return m.Git(args, output)
}

func (m *Mock) Git(args string, output *string) error {
	fmt.Printf("CMD: git %s\n", args)

	m.assert.NotEmpty(m.expectedCmd, fmt.Sprintf("Unexpected command: git %s\n", args))

	expected := m.expectedCmd[0]
	actual := "git " + args
	m.assert.Equal(expected, actual)

	if m.response[0].Valid() {
		m.assert.NotNil(output)
		*output = m.response[0].Output()
	} else {
		m.assert.Nil(output)
	}

	m.expectedCmd = m.expectedCmd[1:]
	m.response = m.response[1:]

	return nil
}

func (m *Mock) ExpectationsMet() {
	m.assert.Empty(m.expectedCmd, fmt.Sprintf("expected additional git commands: %v", m.expectedCmd))
	m.assert.Empty(m.response, fmt.Sprintf("expected additional git responses: %v", m.response))
}

func (m *Mock) MustGit(argStr string, output *string) {
	err := m.Git(argStr, output)
	if err != nil {
		panic(err)
	}
}

func (m *Mock) RootDir() string {
	return ""
}

type Mock struct {
	assert      *require.Assertions
	expectedCmd []string
	response    []responder
}

type responder interface {
	Valid() bool
	Output() string
}

func (m *Mock) ExpectFetch() {
	m.expect("git fetch")
	m.expect("git rebase origin/master --autostash")
}

func (m *Mock) ExpectDeleteBranch(branchName string) {
	m.expect(fmt.Sprintf("git push origin --delete %s", branchName))
}

func (m *Mock) ExpectLogAndRespond(commits []*git.Commit) {
	m.expect("git log --format=medium --no-color origin/master..HEAD").commitRespond(commits)
}

func (m *Mock) ExpectStatus() {
	m.expect("git status --porcelain --untracked-files=no").commitRespond(nil)
}

func (m *Mock) ExpectPushCommits(commits []*git.Commit) {
	m.ExpectStatus()

	var refNames []string
	for _, c := range commits {
		branchName := "spr/master/" + c.CommitID
		refNames = append(refNames, c.CommitHash+":refs/heads/"+branchName)
	}
	m.expect("git push --force --atomic origin " + strings.Join(refNames, " "))
}

func (m *Mock) ExpectRemote(remote string) {
	response := fmt.Sprintf("origin  %s (fetch)\n", remote)
	response += fmt.Sprintf("origin  %s (push)\n", remote)
	m.expect("git remote -v").respond(response)
}

func (m *Mock) ExpectFixup(commitHash string) {
	m.expect("git commit --fixup " + commitHash)
	m.expect("git rebase -i --autosquash --autostash origin/master")
}

func (m *Mock) ExpectLocalBranch(name string) {
	m.expect("git branch --no-color").respond(name)
}

func (m *Mock) expect(cmd string, args ...interface{}) *Mock {
	m.expectedCmd = append(m.expectedCmd, fmt.Sprintf(cmd, args...))
	m.response = append(m.response, &commitResponse{valid: false})
	return m
}

func (m *Mock) respond(response string) {
	m.response[len(m.response)-1] = &stringResponse{
		valid:  true,
		output: response,
	}
}

func (m *Mock) commitRespond(commits []*git.Commit) {
	m.response[len(m.response)-1] = &commitResponse{
		valid:   true,
		commits: commits,
	}
}

type stringResponse struct {
	valid  bool
	output string
}

func (r *stringResponse) Valid() bool {
	return r.valid
}

func (r *stringResponse) Output() string {
	return r.output
}

type commitResponse struct {
	valid   bool
	commits []*git.Commit
}

func (r *commitResponse) Valid() bool {
	return r.valid
}

func (r *commitResponse) Output() string {
	if !r.valid {
		return ""
	}

	var b strings.Builder
	for _, c := range r.commits {
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
