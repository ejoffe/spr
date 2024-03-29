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

	// PullRequestsWithMergeQueue from github/githubclient/queries.graphql:40
	PullRequestsWithMergeQueue(ctx context.Context,
		repoOwner string,
		repoName string,
	) (*PullRequestsWithMergeQueueResponse, error)

	// AssignableUsers from github/githubclient/queries.graphql:82
	AssignableUsers(ctx context.Context,
		repoOwner string,
		repoName string,
		endCursor *string,
	) (*AssignableUsersResponse, error)

	// CreatePullRequest from github/githubclient/queries.graphql:102
	CreatePullRequest(ctx context.Context,
		input CreatePullRequestInput,
	) (*CreatePullRequestResponse, error)

	// UpdatePullRequest from github/githubclient/queries.graphql:115
	UpdatePullRequest(ctx context.Context,
		input UpdatePullRequestInput,
	) (*UpdatePullRequestResponse, error)

	// AddReviewers from github/githubclient/queries.graphql:127
	AddReviewers(ctx context.Context,
		input RequestReviewsInput,
	) (*AddReviewersResponse, error)

	// CommentPullRequest from github/githubclient/queries.graphql:139
	CommentPullRequest(ctx context.Context,
		input AddCommentInput,
	) (*CommentPullRequestResponse, error)

	// MergePullRequest from github/githubclient/queries.graphql:149
	MergePullRequest(ctx context.Context,
		input MergePullRequestInput,
	) (*MergePullRequestResponse, error)

	// AutoMergePullRequest from github/githubclient/queries.graphql:161
	AutoMergePullRequest(ctx context.Context,
		input EnablePullRequestAutoMergeInput,
	) (*AutoMergePullRequestResponse, error)

	// ClosePullRequest from github/githubclient/queries.graphql:173
	ClosePullRequest(ctx context.Context,
		input ClosePullRequestInput,
	) (*ClosePullRequestResponse, error)

	// StarCheck from github/githubclient/queries.graphql:185
	StarCheck(ctx context.Context,
		after *string,
	) (*StarCheckResponse, error)

	// StarGetRepo from github/githubclient/queries.graphql:201
	StarGetRepo(ctx context.Context,
		owner string,
		name string,
	) (*StarGetRepoResponse, error)

	// StarAdd from github/githubclient/queries.graphql:210
	StarAdd(ctx context.Context,
		input AddStarInput,
	) (*StarAddResponse, error)
}

func NewClient(url string, httpclient *http.Client) Client {
	return &gqlclient{
		gql: client.NewGQLClient(url, httpclient),
	}
}

func NewDebugClient(url string, httpclient *http.Client) Client {
	return &gqlclient{
		gql: client.NewGQLClient(url, httpclient, client.WithDebug()),
	}
}

type gqlclient struct {
	gql *client.GQLClient
}
