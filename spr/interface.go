package spr

import "context"

type SPRInterface interface {
	StatusPullRequests(ctx context.Context)
	UpdatePullRequests(ctx context.Context)
	MergePullRequests(ctx context.Context)
}
