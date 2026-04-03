package mockjj

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ejoffe/spr/git"
	"github.com/stretchr/testify/require"
)

// NewMockJj creates a new mock jj executor.
func NewMockJj(t *testing.T) *Mock {
	return &Mock{
		assert: require.New(t),
	}
}

// Mock implements vcs.JjInterface with expectation-based command verification,
// following the same pattern as git/mockgit/mockgit.go.
type Mock struct {
	assert      *require.Assertions
	expectedCmd []string
	response    []responder
}

// Jj verifies the command matches expectations and returns the configured response.
func (m *Mock) Jj(args string, output *string) error {
	fmt.Printf("CMD: jj %s\n", args)

	m.assert.NotEmpty(m.expectedCmd, fmt.Sprintf("Unexpected command: jj %s\n", args))

	expected := m.expectedCmd[0]
	actual := "jj " + args
	m.assert.Equal(expected, actual)

	resp := m.response[0]
	m.expectedCmd = m.expectedCmd[1:]
	m.response = m.response[1:]

	if resp.Err() != nil {
		return resp.Err()
	}
	if resp.Valid() {
		m.assert.NotNil(output)
		*output = resp.Output()
	} else if output != nil {
		*output = ""
	}

	return nil
}

// MustJj calls Jj and panics on error.
func (m *Mock) MustJj(args string, output *string) {
	err := m.Jj(args, output)
	if err != nil {
		panic(err)
	}
}

// JjArgs verifies commands with pre-split arguments.
func (m *Mock) JjArgs(args []string, output *string) error {
	return m.Jj(strings.Join(args, " "), output)
}

// ExpectationsMet verifies all expected commands were called.
func (m *Mock) ExpectationsMet() {
	m.assert.Empty(m.expectedCmd, fmt.Sprintf("expected additional jj commands: %v", m.expectedCmd))
	m.assert.Empty(m.response, fmt.Sprintf("expected additional jj responses: %v", m.response))
}

// ExpectFetch expects a jj git fetch command.
func (m *Mock) ExpectFetch() {
	m.expect("jj git fetch")
}

// ExpectRebase expects a jj rebase command.
func (m *Mock) ExpectRebase(remote, branch string) {
	m.expect(fmt.Sprintf("jj rebase -b @ -d %s@%s", branch, remote))
}

// ExpectLogAndRespond expects the jj log command and returns formatted output.
func (m *Mock) ExpectLogAndRespond(commits []*git.Commit) {
	template := `commit_id ++ "\x1f" ++ change_id ++ "\x1f" ++ empty ++ "\x1f" ++ description ++ "\x1e"`
	m.expect(fmt.Sprintf(`jj log --no-graph --reversed --color=never -r trunk()..@ -T %s`, template)).
		respond(formatJjLogResponse(commits))
}

// ExpectDescribe expects a jj describe command for a specific change.
func (m *Mock) ExpectDescribe(changeID, message string) {
	m.expect(fmt.Sprintf("jj describe -r %s -m %s", changeID, message))
}

// ExpectSquash expects a jj squash --into command.
func (m *Mock) ExpectSquash(changeID string) {
	m.expect(fmt.Sprintf("jj squash --into %s", changeID))
}

// ExpectEdit expects a jj edit command.
func (m *Mock) ExpectEdit(changeID string) {
	m.expect(fmt.Sprintf("jj edit %s", changeID))
}

// ExpectNew expects a jj new command.
func (m *Mock) ExpectNew(changeID string) {
	m.expect(fmt.Sprintf("jj new %s", changeID))
}

// ExpectOpLog expects a jj op log command and returns an operation ID.
func (m *Mock) ExpectOpLog(opID string) {
	m.expect("jj op log --no-graph -n 1 -T id.short(16)").respond(opID)
}

// ExpectOpRestore expects a jj op restore command.
func (m *Mock) ExpectOpRestore(opID string) {
	m.expect(fmt.Sprintf("jj op restore %s", opID))
}

// ExpectLogAt expects a jj log command for the current change ID.
func (m *Mock) ExpectLogAt(changeID string) {
	m.expect("jj log --no-graph -r @ -T change_id").respond(changeID)
}

// ExpectCheckChildren expects the children(@) completeness check and responds
// with the given output (empty string means no children, i.e. @ is at the top).
func (m *Mock) ExpectCheckChildren(response string) {
	m.expect(`jj log --no-graph --color=never -r children(@) & trunk()..@+ -T change_id ++ "\n"`).respond(response)
}

func (m *Mock) expect(cmd string) *Mock {
	m.expectedCmd = append(m.expectedCmd, cmd)
	m.response = append(m.response, &stringResponse{valid: false})
	return m
}

func (m *Mock) respond(response string) {
	m.response[len(m.response)-1] = &stringResponse{
		valid:  true,
		output: response,
	}
}

func (m *Mock) respondWithError(err error) {
	m.response[len(m.response)-1] = &errResponse{err: err}
}

// ExpectDescribeAndFail expects a jj describe command and returns an error.
func (m *Mock) ExpectDescribeAndFail(changeID, message string, err error) {
	m.expect(fmt.Sprintf("jj describe -r %s -m %s", changeID, message)).respondWithError(err)
}

type responder interface {
	Valid() bool
	Output() string
	Err() error
}

type stringResponse struct {
	valid  bool
	output string
}

func (r *stringResponse) Valid() bool    { return r.valid }
func (r *stringResponse) Output() string { return r.output }
func (r *stringResponse) Err() error     { return nil }

type errResponse struct {
	err error
}

func (r *errResponse) Valid() bool    { return false }
func (r *errResponse) Output() string { return "" }
func (r *errResponse) Err() error     { return r.err }

// formatJjLogResponse formats commits as jj log output with field/record separators.
func formatJjLogResponse(commits []*git.Commit) string {
	var b strings.Builder
	for _, c := range commits {
		desc := c.Subject
		if c.Body != "" {
			desc += "\n\n" + c.Body
		}
		if c.CommitID != "" {
			desc += "\n\ncommit-id:" + c.CommitID
		}
		changeID := c.ChangeID
		if changeID == "" {
			changeID = "jjchange_" + c.CommitID
		}
		fmt.Fprintf(&b, "%s\x1f%s\x1ffalse\x1f%s\n\x1e", c.CommitHash, changeID, desc)
	}
	return b.String()
}
