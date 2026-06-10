package spr

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/stretchr/testify/require"
)

// stubStackedDiff builds a stackediff wired with bytes.Buffer for I/O
// (no TTY), an empty config, and the divergencePromptFn left for the
// caller to set (or leave nil to test the non-TTY refusal path).
func stubStackedDiff(policy string) *stackediff {
	cfg := config.EmptyConfig()
	cfg.Repo.OnRemoteDivergence = policy
	return &stackediff{
		config: cfg,
		input:  &bytes.Buffer{},
		output: &bytes.Buffer{},
	}
}

func TestApplyDivergencePolicy_Drop_ProceedsWithSummary(t *testing.T) {
	sd := stubStackedDiff(config.OnRemoteDivergenceDrop)
	out := &bytes.Buffer{}
	sd.output = out

	divs := []git.Divergence{
		{
			HeadBranch:     "spr/master/11111111",
			LocalCommit:    git.Commit{CommitID: "11111111"},
			ForeignCommits: []git.RemoteCommit{{SHA: "sha-M", Subject: "fix"}},
			Reason:         git.DivergenceForeignCommits,
		},
	}
	ok := sd.applyDivergencePolicy(divs)

	require.True(t, ok, "drop policy must proceed")
	require.Contains(t, out.String(), "onRemoteDivergence=drop")
	require.Contains(t, out.String(), "spr/master/11111111")
}

func TestApplyDivergencePolicy_Ask_NonTTY_RefusesWithGuide(t *testing.T) {
	sd := stubStackedDiff(config.OnRemoteDivergenceAsk)
	out := &bytes.Buffer{}
	sd.output = out
	// sd.input is a *bytes.Buffer → isTTY returns false.

	divs := []git.Divergence{
		{
			HeadBranch:     "spr/master/cc11dd22",
			LocalCommit:    git.Commit{CommitID: "cc11dd22"},
			RemoteTipSHA:   "abcdef1234",
			ForeignCommits: []git.RemoteCommit{{SHA: "sha-M1", Subject: "fix typo", Author: "M"}},
			Reason:         git.DivergenceForeignCommits,
		},
	}
	ok := sd.applyDivergencePolicy(divs)

	require.False(t, ok, "non-TTY ask must refuse")
	s := out.String()
	require.Contains(t, s, "spr/master/cc11dd22")
	require.Contains(t, s, "git fetch origin spr/master/cc11dd22",
		"resolution guide must include the fetch command")
	require.Contains(t, s, "git cherry-pick",
		"resolution guide must include the cherry-pick command")
	require.Contains(t, s, "not a terminal",
		"non-TTY refusal must explain why")
	require.Contains(t, s, "onRemoteDivergence: drop",
		"non-TTY refusal must mention the opt-in to silent drop")
}

func TestApplyDivergencePolicy_Ask_WithInjectedPromptDrop(t *testing.T) {
	sd := stubStackedDiff(config.OnRemoteDivergenceAsk)
	var captured []git.Divergence
	sd.divergencePromptFn = func(d []git.Divergence) DivergenceAction {
		captured = d
		return DivergenceActionDrop
	}

	divs := []git.Divergence{{
		HeadBranch:  "spr/master/11111111",
		LocalCommit: git.Commit{CommitID: "11111111"},
		Reason:      git.DivergenceForeignCommits,
	}}
	ok := sd.applyDivergencePolicy(divs)

	require.True(t, ok)
	require.Len(t, captured, 1)
	require.Equal(t, "spr/master/11111111", captured[0].HeadBranch)
}

func TestApplyDivergencePolicy_Ask_WithInjectedPromptAbort(t *testing.T) {
	sd := stubStackedDiff(config.OnRemoteDivergenceAsk)
	sd.divergencePromptFn = func(d []git.Divergence) DivergenceAction {
		return DivergenceActionAbort
	}
	ok := sd.applyDivergencePolicy([]git.Divergence{{Reason: git.DivergenceForeignCommits}})
	require.False(t, ok)
}

// (The Stage 1 test asserting that `merge` falls through to `ask` with a
// "Stage 2 reserved" note is gone now that this PR implements the actual
// fold. New behavior is covered in divergence_fold_test.go.)

func TestApplyDivergencePolicy_EmptyPolicy_DefaultsToAsk(t *testing.T) {
	sd := stubStackedDiff("") // empty config value
	sd.divergencePromptFn = func(d []git.Divergence) DivergenceAction {
		return DivergenceActionAbort
	}
	ok := sd.applyDivergencePolicy([]git.Divergence{{Reason: git.DivergenceForeignCommits}})
	require.False(t, ok, "empty policy treated as ask, and our prompt returned abort")
}

func TestApplyDivergencePolicy_InvalidPolicy_Refuses(t *testing.T) {
	sd := stubStackedDiff("nonsense")
	out := &bytes.Buffer{}
	sd.output = out
	ok := sd.applyDivergencePolicy([]git.Divergence{{Reason: git.DivergenceForeignCommits}})
	require.False(t, ok)
	require.Contains(t, out.String(), "invalid onRemoteDivergence")
}

func TestInteractiveDivergencePrompt_DropResponse(t *testing.T) {
	in := strings.NewReader("d\n")
	out := &bytes.Buffer{}
	action := interactiveDivergencePrompt(in, out, []git.Divergence{
		{HeadBranch: "spr/master/11111111", LocalCommit: git.Commit{CommitID: "11111111"},
			Reason: git.DivergenceForeignCommits},
	})
	require.Equal(t, DivergenceActionDrop, action)
}

func TestInteractiveDivergencePrompt_DefaultIsAbort(t *testing.T) {
	in := strings.NewReader("\n") // user just hits enter
	out := &bytes.Buffer{}
	action := interactiveDivergencePrompt(in, out, []git.Divergence{
		{HeadBranch: "spr/master/11111111", Reason: git.DivergenceForeignCommits},
	})
	require.Equal(t, DivergenceActionAbort, action, "enter without 'd' must abort")
}

func TestInteractiveDivergencePrompt_AnyOtherInputAborts(t *testing.T) {
	in := strings.NewReader("yes please\n")
	out := &bytes.Buffer{}
	action := interactiveDivergencePrompt(in, out, []git.Divergence{
		{Reason: git.DivergenceForeignCommits},
	})
	require.Equal(t, DivergenceActionAbort, action)
}

func TestPrintResolutionGuide_CidMismatchPath(t *testing.T) {
	out := &bytes.Buffer{}
	printResolutionGuide(out, []git.Divergence{{
		HeadBranch:  "spr/master/aaaaaaaa",
		LocalCommit: git.Commit{CommitID: "aaaaaaaa"},
		MismatchCID: "99999999",
		Reason:      git.DivergenceCidMismatch,
	}})
	s := out.String()
	require.Contains(t, s, "unexpected commit-id 99999999")
	require.Contains(t, s, "Inspect manually",
		"mismatch case should ask for human inspection, not auto-fold")
}

func TestPrintResolutionGuide_CidNotFoundPath(t *testing.T) {
	out := &bytes.Buffer{}
	printResolutionGuide(out, []git.Divergence{{
		HeadBranch:  "spr/master/aaaaaaaa",
		LocalCommit: git.Commit{CommitID: "aaaaaaaa"},
		Reason:      git.DivergenceCidNotFound,
	}})
	s := out.String()
	require.Contains(t, s, "deep rewrite")
	require.Contains(t, s, "Inspect manually")
}
