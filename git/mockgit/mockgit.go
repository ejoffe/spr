package mockgit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ejoffe/spr/git"
	"github.com/stretchr/testify/require"
)

// NewMockGit creates and new mock git instance
func NewMockGit(t *testing.T) *Mock {
	return &Mock{
		assert:  require.New(t),
		rootdir: "",
	}
}

// SetRootDir sets the root directory returned by RootDir()
func (m *Mock) SetRootDir(dir string) {
	m.rootdir = dir
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

	err := m.errors[0]

	m.expectedCmd = m.expectedCmd[1:]
	m.response = m.response[1:]
	m.errors = m.errors[1:]

	return err
}

func (m *Mock) DeleteRemoteBranch(ctx context.Context, branch string) error {
	return m.Git(fmt.Sprintf("DeleteRemoteBranch(%s)", branch), nil)
}

func (m *Mock) ExpectationsMet() {
	m.assert.Empty(m.expectedCmd, fmt.Sprintf("expected additional git commands: %v", m.expectedCmd))
	m.assert.Empty(m.response, fmt.Sprintf("expected additional git responses: %v", m.response))
	m.assert.Empty(m.errors, fmt.Sprintf("expected additional git errors: %v", m.errors))
}

func (m *Mock) MustGit(argStr string, output *string) {
	err := m.Git(argStr, output)
	if err != nil {
		panic(err)
	}
}

func (m *Mock) RootDir() string {
	return m.rootdir
}

type Mock struct {
	assert      *require.Assertions
	expectedCmd []string
	response    []responder
	errors      []error
	rootdir     string
}

type responder interface {
	Valid() bool
	Output() string
}

func (m *Mock) ExpectFetch() {
	m.expect("git fetch")
	m.expect("git rebase origin/master --autostash")
}

func (m *Mock) ExpectNoFetch() {
	m.expect("git rebase origin/master --autostash")
}

func (m *Mock) ExpectDeleteBranch(branchName string) {
	m.expect(fmt.Sprintf("git DeleteRemoteBranch(%s)", branchName))
}

// ExpectEditStart expects the interactive rebase command used to start an edit session
func (m *Mock) ExpectEditStart() {
	m.expect("git rebase -i --autostash origin/master")
}

// ExpectEditDoneAmend expects the amend + rebase continue sequence for a successful edit --done
func (m *Mock) ExpectEditDoneAmend() {
	m.expect("git add -u")
	m.expect("git commit --amend --no-edit")
	m.expect("git rebase --continue")
}

// ExpectEditDoneAmendWithConflict expects amend succeeds but rebase --continue fails (conflict)
func (m *Mock) ExpectEditDoneAmendWithConflict() {
	m.expect("git add -u")
	m.expect("git commit --amend --no-edit")
	m.expectError("git rebase --continue", errors.New("conflict"))
}

// ExpectEditDoneConflictResolved expects the conflict resolution path (no amend, just rebase continue)
func (m *Mock) ExpectEditDoneConflictResolved() {
	m.expect("git add -u")
	m.expect("git rebase --continue")
}

// ExpectEditAbort expects the rebase abort command
func (m *Mock) ExpectEditAbort() {
	m.expect("git rebase --abort")
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
	m.expect("git push --force-with-lease --atomic origin " + strings.Join(refNames, " "))
}

func (m *Mock) ExpectRemote(remote string) {
	response := fmt.Sprintf("origin  %s (fetch)\n", remote)
	response += fmt.Sprintf("origin  %s (push)\n", remote)
	m.expect("git remote -v").respond(response)
}

// ExpectFetchHeadRefs queues an expectation for a batched explicit-refspec
// fetch of PR head branches: `git fetch <remote> <branch>:refs/remotes/<remote>/<branch> ...`.
// Used by the maintainer-edit divergence detection so a restrictive
// remote.origin.fetch refspec can't silently disable detection.
func (m *Mock) ExpectFetchHeadRefs(remote string, branches []string) {
	refspecs := make([]string, len(branches))
	for i, b := range branches {
		refspecs[i] = b + ":refs/remotes/" + remote + "/" + b
	}
	m.expect("git fetch " + remote + " " + strings.Join(refspecs, " "))
}

// remoteHeadLogLiteralCmd returns the literal git command divergence
// detection issues for one head ref (the same string that realgit's Git()
// would receive).
func remoteHeadLogLiteralCmd(remote, branch string, depth int) string {
	ref := "refs/remotes/" + remote + "/" + branch
	return fmt.Sprintf("git log -z --no-color --format=%%H%%n%%an%%n%%s%%n%%b -n %d %s", depth, ref)
}

// ExpectRemoteHeadLog queues an expectation for the `git log -z` call used
// by divergence detection on a single remote head ref, and responds with
// canned output formatted the way real git would.
func (m *Mock) ExpectRemoteHeadLog(remote, branch string, depth int, commits []RemoteCommitFixture) {
	var parts []string
	for _, c := range commits {
		parts = append(parts, c.SHA+"\n"+c.Author+"\n"+c.Subject+"\n"+c.Body)
	}
	// expect() re-Sprintfs the cmd, so escape % → %%.
	escaped := strings.ReplaceAll(remoteHeadLogLiteralCmd(remote, branch, depth), "%", "%%")
	m.expect(escaped).respond(strings.Join(parts, "\x00"))
}

// ExpectRemoteHeadLogMissing queues an expectation that the log call for
// the given branch returns an error (ref absent on remote). Detection
// treats this as "PR not yet pushed".
func (m *Mock) ExpectRemoteHeadLogMissing(remote, branch string, depth int) {
	// readSingleHead passes &output even when the call may fail, so use
	// the variant that tolerates a non-nil output pointer.
	m.expectErrorWithEmptyOutput(remoteHeadLogLiteralCmd(remote, branch, depth),
		errors.New("fatal: bad revision"))
}

// RemoteCommitFixture carries the minimum data needed to synthesise one
// commit entry in the `git log -z` output for a remote head walk.
type RemoteCommitFixture struct {
	SHA     string
	Author  string
	Subject string
	Body    string
}

// ExpectDivergenceCheckClean queues the divergence-detection command
// sequence (one batched fetch + one log -z per commit) and responds
// with histories that exactly match the local commits — i.e. no
// maintainer activity, no divergence flagged. Use this in tests that
// exercise `spr update` and don't care about the divergence layer.
//
// The remote is hard-coded to "origin" and the branch prefix to
// "spr/master/" to match the mockgit assumptions used elsewhere in
// this file.
func (m *Mock) ExpectDivergenceCheckClean(commits []*git.Commit) {
	if len(commits) == 0 {
		return
	}
	// Callers pass commits in git-log order (newest first), matching
	// ExpectLogAndRespond. GetLocalCommitStack/parseLocalCommitStack
	// reverses that to oldest-first before passing to checkRemoteDivergence,
	// so we mirror that reversal here when building branch expectations.
	reversed := make([]*git.Commit, len(commits))
	for i, c := range commits {
		reversed[len(commits)-1-i] = c
	}
	branches := make([]string, len(reversed))
	for i, c := range reversed {
		branches[i] = "spr/master/" + c.CommitID
	}
	m.ExpectFetchHeadRefs("origin", branches)
	for _, c := range reversed {
		m.ExpectRemoteHeadLog("origin", "spr/master/"+c.CommitID, 20,
			[]RemoteCommitFixture{
				{
					SHA:     c.CommitHash,
					Subject: c.Subject,
					Body:    "commit-id: " + c.CommitID,
				},
			})
	}
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
	m.errors = append(m.errors, nil)
	return m
}

func (m *Mock) expectError(cmd string, err error) *Mock {
	m.expectedCmd = append(m.expectedCmd, cmd)
	m.response = append(m.response, &commitResponse{valid: false})
	m.errors = append(m.errors, err)
	return m
}

// expectErrorWithEmptyOutput is like expectError but tolerates a non-nil
// output pointer on the caller side (the real-git path may pass a buffer
// even for commands that can fail). The buffer is set to "" and the error
// is returned.
func (m *Mock) expectErrorWithEmptyOutput(cmd string, err error) *Mock {
	m.expectedCmd = append(m.expectedCmd, cmd)
	m.response = append(m.response, &stringResponse{valid: true, output: ""})
	m.errors = append(m.errors, err)
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
