// Tests in package git_test rather than git so they can use git/mockgit
// without an import cycle.
package git_test

import (
	"testing"

	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/git/mockgit"
	"github.com/stretchr/testify/require"
)

func TestGetRemoteURL(t *testing.T) {
	t.Run("returns trimmed URL on success", func(t *testing.T) {
		mock := mockgit.NewMockGit(t)
		mock.ExpectRemoteGetURL("origin", "git@github.com:r2/d2.git")

		url, err := git.GetRemoteURL(mock, "origin")

		require.NoError(t, err)
		require.Equal(t, "git@github.com:r2/d2.git", url)
		mock.ExpectationsMet()
	})

	t.Run("passes the remote name through verbatim", func(t *testing.T) {
		mock := mockgit.NewMockGit(t)
		mock.ExpectRemoteGetURL("upstream", "https://github.com/ejoffe/spr.git")

		url, err := git.GetRemoteURL(mock, "upstream")

		require.NoError(t, err)
		require.Equal(t, "https://github.com/ejoffe/spr.git", url)
		mock.ExpectationsMet()
	})
}
