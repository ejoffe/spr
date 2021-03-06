// Code generated by github.com/inigolabs/fezzik, DO NOT EDIT.

package genclient

import (
	"context"
	"net/http"

	"github.com/inigolabs/fezzik/client"
)

type Client interface {
	// PullRequests from github/githubclient/queries.graphql:1
	PullRequests(ctx context.Context,
		repoOwner string,
		repoName string,
	) (*PullRequestsResponse, error)

	// AssignableUsers from github/githubclient/queries.graphql:39
	AssignableUsers(ctx context.Context,
		repoOwner string,
		repoName string,
		endCursor *string,
	) (*AssignableUsersResponse, error)

	// CreatePullRequest from github/githubclient/queries.graphql:59
	CreatePullRequest(ctx context.Context,
		input CreatePullRequestInput,
	) (*CreatePullRequestResponse, error)

	// UpdatePullRequest from github/githubclient/queries.graphql:72
	UpdatePullRequest(ctx context.Context,
		input UpdatePullRequestInput,
	) (*UpdatePullRequestResponse, error)

	// AddReviewers from github/githubclient/queries.graphql:84
	AddReviewers(ctx context.Context,
		input RequestReviewsInput,
	) (*AddReviewersResponse, error)

	// CommentPullRequest from github/githubclient/queries.graphql:96
	CommentPullRequest(ctx context.Context,
		input AddCommentInput,
	) (*CommentPullRequestResponse, error)

	// MergePullRequest from github/githubclient/queries.graphql:106
	MergePullRequest(ctx context.Context,
		input MergePullRequestInput,
	) (*MergePullRequestResponse, error)

	// ClosePullRequest from github/githubclient/queries.graphql:118
	ClosePullRequest(ctx context.Context,
		input ClosePullRequestInput,
	) (*ClosePullRequestResponse, error)

	// StarCheck from github/githubclient/queries.graphql:130
	StarCheck(ctx context.Context,
		after *string,
	) (*StarCheckResponse, error)

	// StarGetRepo from github/githubclient/queries.graphql:146
	StarGetRepo(ctx context.Context,
		owner string,
		name string,
	) (*StarGetRepoResponse, error)

	// StarAdd from github/githubclient/queries.graphql:155
	StarAdd(ctx context.Context,
		input AddStarInput,
	) (*StarAddResponse, error)
}

func NewClient(url string, httpclient *http.Client) Client {
	return &gqlclient{
		gql: client.NewGQLClient(url, httpclient),
	}
}

type gqlclient struct {
	gql *client.GQLClient
}
