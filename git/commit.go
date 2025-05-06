package git

import "context"

type GitInterface interface {
	GitWithEditor(args string, output *string, editorCmd string) error
	Git(args string, output *string) error
	MustGit(args string, output *string)
	RootDir() string
	DeleteRemoteBranch(ctx context.Context, branch string) error
}

// Commit has all the git commit info
type Commit struct {
	// CommitID is a long lasting id describing the commit.
	//  The CommitID is generated and added to the end of the commit message on the initial commit.
	//  The CommitID remains the same when a commit is amended.
	CommitID string

	// CommitHash is the git commit hash, this gets updated everytime the commit is amended.
	CommitHash string

	// Subject is the subject of the commit message.
	Subject string

	// Body is the body of the commit message.
	Body string

	// WIP is true if the commit is still work in progress.
	WIP bool
}
