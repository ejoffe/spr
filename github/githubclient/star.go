package githubclient

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ejoffe/rake"
	"github.com/ejoffe/spr/config"
	"github.com/shurcooL/githubv4"
)

const (
	sprRepoOwner = "ejoffe"
	sprRepoName  = "spr"
	sprRepo      = "ejoffe/spr"
	promptCycle  = 50
)

func (c *client) MaybeStar(ctx context.Context, cfg *config.Config) {
	if !cfg.User.Stargazer && cfg.User.RunCount%promptCycle == 0 {
		if c.isStar(ctx) {
			cfg.User.Stargazer = true
			rake.LoadSources(cfg.User,
				rake.YamlFileWriter(config.UserConfigFilePath()))
		} else {
			fmt.Print("enjoying git spr? add a GitHub star? [Y/n]:")
			reader := bufio.NewReader(os.Stdin)
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)
			if line != "n" {
				c.addStar(ctx)
				cfg.User.Stargazer = true
				rake.LoadSources(cfg.User,
					rake.YamlFileWriter(config.UserConfigFilePath()))
				fmt.Println("Thank you! Happy Coding!")
			}
		}
	}
}

func (c *client) isStar(ctx context.Context) bool {
	type queryType struct {
		Viewer struct {
			StarredRepositories struct {
				Nodes []struct {
					NameWithOwner string
				}
				Edges []struct {
					Cursor string
				}
				TotalCount int
			} `graphql:"starredRepositories(first: 100, after:$after)"`
		}
	}

	iteration := 0
	cursor := ""
	for {
		var query queryType
		variables := map[string]interface{}{
			"after": githubv4.String(cursor),
		}
		err := c.api.Query(ctx, &query, variables)
		check(err)

		edgeCount := len(query.Viewer.StarredRepositories.Edges)
		if edgeCount == 0 {
			return false
		}

		for _, node := range query.Viewer.StarredRepositories.Nodes {
			if node.NameWithOwner == sprRepo {
				return true
			}
		}

		cursor = query.Viewer.StarredRepositories.Edges[edgeCount-1].Cursor

		iteration++
		if iteration > 10 {
			// too many stars in the sky
			return false
		}
	}
}

func (c *client) addStar(ctx context.Context) {
	var repo struct {
		Repository struct {
			ID string
		} `graphql:"repository(owner: $owner, name: $name)"`
	}
	repoVariables := map[string]interface{}{
		"owner": githubv4.String(sprRepoOwner),
		"name":  githubv4.String(sprRepoName),
	}
	err := c.api.Query(ctx, &repo, repoVariables)
	check(err)

	var star struct {
		AddStar struct {
			ClientMutationID string
		} `graphql:"addStar(input: $input)"`
	}
	input := githubv4.AddStarInput{
		StarrableID: repo.Repository.ID,
	}
	err = c.api.Mutate(ctx, &star, input, nil)
	check(err)
}
