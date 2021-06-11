package spr

import (
	"bytes"
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/stretchr/testify/assert"
)

func TestParseLocalCommitStack(t *testing.T) {
	var buffer bytes.Buffer
	sd := NewStackedPR(&config.Config{}, nil, &buffer, false)
	tests := []struct {
		name                      string
		inputCommitLog            string
		expectedCommits           []git.Commit
		expectedCommitHookMessage bool
	}{
		{
			name: "SingleValidCommitNoBody",
			inputCommitLog: `
commit d89e0e460ed817c81641f32b1a506b60164b4403 (HEAD -> master)
Author: Han Solo
Date:   Wed May 21 19:53:12 1980 -0700

	Supergalactic speed

	commit-id:053f6d16
`,
			expectedCommits: []git.Commit{
				{
					CommitHash: "d89e0e460ed817c81641f32b1a506b60164b4403",
					CommitID:   "053f6d16",
					Subject:    "Supergalactic speed",
					Body:       "",
				},
			},
			expectedCommitHookMessage: false,
		},
		{
			name: "SingleValidCommitWithBody",
			inputCommitLog: `
commit d89e0e460ed817c81641f32b1a506b60164b4403 (HEAD -> master)
Author: Han Solo
Date:   Wed May 21 19:53:12 1980 -0700

	Supergalactic speed

	Super universe body.

	commit-id:053f6d16
`,
			expectedCommits: []git.Commit{
				{
					CommitHash: "d89e0e460ed817c81641f32b1a506b60164b4403",
					CommitID:   "053f6d16",
					Subject:    "Supergalactic speed",
					Body:       "Super universe body.",
				},
			},
			expectedCommitHookMessage: false,
		},
		{
			name: "TwoValidCommitsNoBody",
			inputCommitLog: `
commit d89e0e460ed817c81641f32b1a506b60164b4403 (HEAD -> master)
Author: Han Solo
Date:   Wed May 21 19:53:12 1980 -0700

	Supergalactic speed

	commit-id:053f6d16

commit d604099d6604949e786e3d781919d43e46e88521 (origin/pr/ejoffe/master/39c84ea3)
Author: Hans Solo
Date:   Wed May 21 19:52:51 1980 -0700

	More engine power

	commit-id:39c84ea3
`,
			expectedCommits: []git.Commit{
				{
					CommitHash: "d604099d6604949e786e3d781919d43e46e88521",
					CommitID:   "39c84ea3",
					Subject:    "More engine power",
				},
				{
					CommitHash: "d89e0e460ed817c81641f32b1a506b60164b4403",
					CommitID:   "053f6d16",
					Subject:    "Supergalactic speed",
				},
			},
			expectedCommitHookMessage: false,
		},
		{
			name: "SingleCommitMissingCommitID",
			inputCommitLog: `
commit d89e0e460ed817c81641f32b1a506b60164b4403 (HEAD -> master)
Author: Han Solo
Date:   Wed May 21 19:53:12 1980 -0700

	Supergalactic speed
`,
			expectedCommits:           nil,
			expectedCommitHookMessage: true,
		},
	}

	for _, tc := range tests {
		actualCommits := sd.parseLocalCommitStack(tc.inputCommitLog)
		assert.Equal(t, tc.expectedCommits, actualCommits, tc.name)

		if tc.expectedCommitHookMessage {
			assert.Equal(t, buffer.String(), commitInstallHelper[1:])
		} else {
			assert.Equal(t, buffer.Len(), 0)
		}
	}
}
