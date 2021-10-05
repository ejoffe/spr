package github_client

import (
	"net/http"

	"github.com/machinebox/graphql"
)

func NewClient(url string, httpclient *http.Client) *Client {
	if httpclient != nil {
		return &Client{
			gql: graphql.NewClient(url, graphql.WithHTTPClient(httpclient)),
		}
	} else {
		return &Client{
			gql: graphql.NewClient(url),
		}
	}
}

type Client struct {
	gql *graphql.Client
}
