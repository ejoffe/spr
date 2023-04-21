package git

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLocalCommitStack(t *testing.T) {
	var buffer bytes.Buffer
	tests := []struct {
		name            string
		inputCommitLog  string
		expectedCommits []Commit
		expectedValid   bool
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
			expectedCommits: []Commit{
				{
					CommitHash: "d89e0e460ed817c81641f32b1a506b60164b4403",
					CommitID:   "053f6d16",
					Subject:    "Supergalactic speed",
					Body:       "",
				},
			},
			expectedValid: true,
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
			expectedCommits: []Commit{
				{
					CommitHash: "d89e0e460ed817c81641f32b1a506b60164b4403",
					CommitID:   "053f6d16",
					Subject:    "Supergalactic speed",
					Body:       "Super universe body.",
				},
			},
			expectedValid: true,
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
			expectedCommits: []Commit{
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
			expectedValid: true,
		},
		{
			name: "SingleCommitMissingCommitID",
			inputCommitLog: `
commit d89e0e460ed817c81641f32b1a506b60164b4403 (HEAD -> master)
Author: Han Solo
Date:   Wed May 21 19:53:12 1980 -0700

	Supergalactic speed
`,
			expectedCommits: nil,
			expectedValid:   false,
		},
	}

	for _, tc := range tests {
		actualCommits, valid := parseLocalCommitStack(tc.inputCommitLog)
		assert.Equal(t, tc.expectedCommits, actualCommits, tc.name)
		assert.Equal(t, tc.expectedValid, valid, tc.name)
		if tc.expectedValid {
			assert.Equal(t, buffer.Len(), 0, tc.name)
		}
	}
}
