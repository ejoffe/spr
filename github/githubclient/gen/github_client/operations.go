package github_client

import (
	"context"

	"github.com/machinebox/graphql"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
)

var CreatePullRequestOperation string = `
mutation CreatePullRequest($input : CreatePullRequestInput!) {
   createPullRequest(input: $input) {
      pullRequest {
         id
         number
      }
   }
}
`

type CreatePullRequestInputArgs struct {
	Input CreatePullRequestInput `json:"input"`
}

type CreatePullRequestResponse struct {
	CreatePullRequest *struct {
		PullRequest *struct {
			Id     string
			Number int
		}
	}
}

func (c *Client) CreatePullRequest(ctx context.Context, input *CreatePullRequestInputArgs) (
	*CreatePullRequestResponse, error) {

	q := graphql.NewRequest(CreatePullRequestOperation)
	q.Var("input", input.Input)
	var resp map[string]interface{}
	err := c.gql.Run(ctx, q, &resp)
	log.Debug().Interface("resp", resp).Err(err).Msg("CreatePullRequest")
	if err != nil {
		return nil, err
	}

	output := CreatePullRequestResponse{}
	err = mapstructure.Decode(resp, &output)
	log.Debug().Interface("output", output).Err(err).Msg("CreatePullRequest")
	if err != nil {
		return nil, err
	}
	return &output, err
}

var UpdatePullRequestOperation string = `
mutation UpdatePullRequest($input : UpdatePullRequestInput!) {
   updatePullRequest(input: $input) {
      pullRequest {
         number
      }
   }
}
`

type UpdatePullRequestInputArgs struct {
	Input UpdatePullRequestInput `json:"input"`
}

type UpdatePullRequestResponse struct {
	UpdatePullRequest *struct {
		PullRequest *struct {
			Number int
		}
	}
}

func (c *Client) UpdatePullRequest(ctx context.Context, input *UpdatePullRequestInputArgs) (
	*UpdatePullRequestResponse, error) {

	q := graphql.NewRequest(UpdatePullRequestOperation)
	q.Var("input", input.Input)
	var resp map[string]interface{}
	err := c.gql.Run(ctx, q, &resp)
	log.Debug().Interface("resp", resp).Err(err).Msg("UpdatePullRequest")
	if err != nil {
		return nil, err
	}

	output := UpdatePullRequestResponse{}
	err = mapstructure.Decode(resp, &output)
	log.Debug().Interface("output", output).Err(err).Msg("UpdatePullRequest")
	if err != nil {
		return nil, err
	}
	return &output, err
}

var CommentPullRequestOperation string = `
mutation CommentPullRequest($input : AddCommentInput!) {
   addComment(input: $input) {
      clientMutationId
   }
}
`

type CommentPullRequestInputArgs struct {
	Input AddCommentInput `json:"input"`
}

type CommentPullRequestResponse struct {
	AddComment *struct {
		ClientMutationId *string
	}
}

func (c *Client) CommentPullRequest(ctx context.Context, input *CommentPullRequestInputArgs) (
	*CommentPullRequestResponse, error) {

	q := graphql.NewRequest(CommentPullRequestOperation)
	q.Var("input", input.Input)
	var resp map[string]interface{}
	err := c.gql.Run(ctx, q, &resp)
	log.Debug().Interface("resp", resp).Err(err).Msg("CommentPullRequest")
	if err != nil {
		return nil, err
	}

	output := CommentPullRequestResponse{}
	err = mapstructure.Decode(resp, &output)
	log.Debug().Interface("output", output).Err(err).Msg("CommentPullRequest")
	if err != nil {
		return nil, err
	}
	return &output, err
}

var MergePullRequestOperation string = `
mutation MergePullRequest($input : MergePullRequestInput!) {
   mergePullRequest(input: $input) {
      pullRequest {
         number
      }
   }
}
`

type MergePullRequestInputArgs struct {
	Input MergePullRequestInput `json:"input"`
}

type MergePullRequestResponse struct {
	MergePullRequest *struct {
		PullRequest *struct {
			Number int
		}
	}
}

func (c *Client) MergePullRequest(ctx context.Context, input *MergePullRequestInputArgs) (
	*MergePullRequestResponse, error) {

	q := graphql.NewRequest(MergePullRequestOperation)
	q.Var("input", input.Input)
	var resp map[string]interface{}
	err := c.gql.Run(ctx, q, &resp)
	log.Debug().Interface("resp", resp).Err(err).Msg("MergePullRequest")
	if err != nil {
		return nil, err
	}

	output := MergePullRequestResponse{}
	err = mapstructure.Decode(resp, &output)
	log.Debug().Interface("output", output).Err(err).Msg("MergePullRequest")
	if err != nil {
		return nil, err
	}
	return &output, err
}

var ClosePullRequestOperation string = `
mutation ClosePullRequest($input : ClosePullRequestInput!) {
   closePullRequest(input: $input) {
      pullRequest {
         number
      }
   }
}
`

type ClosePullRequestInputArgs struct {
	Input ClosePullRequestInput `json:"input"`
}

type ClosePullRequestResponse struct {
	ClosePullRequest *struct {
		PullRequest *struct {
			Number int
		}
	}
}

func (c *Client) ClosePullRequest(ctx context.Context, input *ClosePullRequestInputArgs) (
	*ClosePullRequestResponse, error) {

	q := graphql.NewRequest(ClosePullRequestOperation)
	q.Var("input", input.Input)
	var resp map[string]interface{}
	err := c.gql.Run(ctx, q, &resp)
	log.Debug().Interface("resp", resp).Err(err).Msg("ClosePullRequest")
	if err != nil {
		return nil, err
	}

	output := ClosePullRequestResponse{}
	err = mapstructure.Decode(resp, &output)
	log.Debug().Interface("output", output).Err(err).Msg("ClosePullRequest")
	if err != nil {
		return nil, err
	}
	return &output, err
}

var AddStarOperation string = `
mutation AddStar($input : AddStarInput!) {
   addStar(input: $input) {
      clientMutationId
   }
}
`

type AddStarInputArgs struct {
	Input AddStarInput `json:"input"`
}

type AddStarResponse struct {
	AddStar *struct {
		ClientMutationId *string
	}
}

func (c *Client) AddStar(ctx context.Context, input *AddStarInputArgs) (
	*AddStarResponse, error) {

	q := graphql.NewRequest(AddStarOperation)
	q.Var("input", input.Input)
	var resp map[string]interface{}
	err := c.gql.Run(ctx, q, &resp)
	log.Debug().Interface("resp", resp).Err(err).Msg("AddStar")
	if err != nil {
		return nil, err
	}

	output := AddStarResponse{}
	err = mapstructure.Decode(resp, &output)
	log.Debug().Interface("output", output).Err(err).Msg("AddStar")
	if err != nil {
		return nil, err
	}
	return &output, err
}

var GetInfoOperation string = `
query GetInfo($repo_owner : String!, $repo_name : String!) {
   viewer {
      login
      pullRequests(first: 100, states: [OPEN]) {
         nodes {
            id
            number
            title
            baseRefName
            headRefName
            mergeable
            reviewDecision
            repository {
               id
            }
            commits(first: 100) {
               nodes {
                  commit {
                     oid
                     messageHeadline
                     messageBody
                     statusCheckRollup {
                        state
                     }
                  }
               }
            }
         }
      }
   }
   repository(owner: $repo_owner, name: $repo_name) {
      id
   }
}
`

type GetInfoInputArgs struct {
	Repo_owner string `json:"repo_owner"`
	Repo_name  string `json:"repo_name"`
}

type GetInfoResponse struct {
	Viewer struct {
		Login        string
		PullRequests struct {
			Nodes []*struct {
				Id             string
				Number         int
				Title          string
				BaseRefName    string
				HeadRefName    string
				Mergeable      MergeableState
				ReviewDecision *PullRequestReviewDecision
				Repository     struct {
					Id string
				}
				Commits struct {
					Nodes []*struct {
						Commit struct {
							Oid               string
							MessageHeadline   string
							MessageBody       string
							StatusCheckRollup *struct {
								State StatusState
							}
						}
					}
				}
			}
		}
	}
	Repository *struct {
		Id string
	}
}

func (c *Client) GetInfo(ctx context.Context, input *GetInfoInputArgs) (
	*GetInfoResponse, error) {

	q := graphql.NewRequest(GetInfoOperation)
	q.Var("repo_owner", input.Repo_owner)
	q.Var("repo_name", input.Repo_name)
	var resp map[string]interface{}
	err := c.gql.Run(ctx, q, &resp)
	log.Debug().Interface("resp", resp).Err(err).Msg("GetInfo")
	if err != nil {
		return nil, err
	}

	output := GetInfoResponse{}
	err = mapstructure.Decode(resp, &output)
	log.Debug().Interface("output", output).Err(err).Msg("GetInfo")
	if err != nil {
		return nil, err
	}
	return &output, err
}

var RepositoryOperation string = `
query Repository($owner : String!, $name : String!) {
   repository(owner: $owner, name: $name) {
      id
   }
}
`

type RepositoryInputArgs struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

type RepositoryResponse struct {
	Repository *struct {
		Id string
	}
}

func (c *Client) Repository(ctx context.Context, input *RepositoryInputArgs) (
	*RepositoryResponse, error) {

	q := graphql.NewRequest(RepositoryOperation)
	q.Var("owner", input.Owner)
	q.Var("name", input.Name)
	var resp map[string]interface{}
	err := c.gql.Run(ctx, q, &resp)
	log.Debug().Interface("resp", resp).Err(err).Msg("Repository")
	if err != nil {
		return nil, err
	}

	output := RepositoryResponse{}
	err = mapstructure.Decode(resp, &output)
	log.Debug().Interface("output", output).Err(err).Msg("Repository")
	if err != nil {
		return nil, err
	}
	return &output, err
}

var IsStarredOperation string = `
query IsStarred($after : String!) {
   viewer {
      starredRepositories(first: 100, after: $after) {
         nodes {
            nameWithOwner
         }
         edges {
            cursor
         }
         totalCount
      }
   }
}
`

type IsStarredInputArgs struct {
	After string `json:"after"`
}

type IsStarredResponse struct {
	Viewer struct {
		StarredRepositories struct {
			Nodes []*struct {
				NameWithOwner string
			}
			Edges []*struct {
				Cursor string
			}
			TotalCount int
		}
	}
}

func (c *Client) IsStarred(ctx context.Context, input *IsStarredInputArgs) (
	*IsStarredResponse, error) {

	q := graphql.NewRequest(IsStarredOperation)
	q.Var("after", input.After)
	var resp map[string]interface{}
	err := c.gql.Run(ctx, q, &resp)
	log.Debug().Interface("resp", resp).Err(err).Msg("IsStarred")
	if err != nil {
		return nil, err
	}

	output := IsStarredResponse{}
	err = mapstructure.Decode(resp, &output)
	log.Debug().Interface("output", output).Err(err).Msg("IsStarred")
	if err != nil {
		return nil, err
	}
	return &output, err
}
