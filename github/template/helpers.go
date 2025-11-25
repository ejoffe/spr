package template

import (
	"bytes"
	"fmt"

	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
)

func AddManualMergeNotice(body string) string {
	return body + "\n\n" +
		"⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). " +
		"Do not merge manually using the UI - doing so may have unexpected results.*"
}

func FormatStackMarkdown(commit git.Commit, stack []*github.PullRequest, showPrTitlesInStack bool) string {
	var buf bytes.Buffer
	for i := len(stack) - 1; i >= 0; i-- {
		isCurrent := stack[i].Commit == commit
		var suffix string
		if isCurrent {
			suffix = " ⬅"
		} else {
			suffix = ""
		}
		var prTitle string
		if showPrTitlesInStack {
			prTitle = fmt.Sprintf("%s ", stack[i].Title)
		} else {
			prTitle = ""
		}

		buf.WriteString(fmt.Sprintf("- %s#%d%s\n", prTitle, stack[i].Number, suffix))
	}

	return buf.String()
}
