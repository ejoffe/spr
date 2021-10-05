package githubclient

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ejoffe/rake"
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/github/githubclient/gen/github_client"
)

const (
	sprRepoOwner = "ejoffe"
	sprRepoName  = "spr"
	sprRepo      = "ejoffe/spr"
	promptCycle  = 25
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
	iteration := 0
	cursor := ""
	for {
		resp, err := c.api.IsStarred(ctx, &github_client.IsStarredInputArgs{
			After: cursor,
		})
		check(err)

		edgeCount := len(resp.Viewer.StarredRepositories.Edges)
		if edgeCount == 0 {
			return false
		}

		for _, node := range resp.Viewer.StarredRepositories.Nodes {
			if node.NameWithOwner == sprRepo {
				return true
			}
		}

		cursor = resp.Viewer.StarredRepositories.Edges[edgeCount-1].Cursor

		iteration++
		if iteration > 10 {
			// too many stars in the sky
			return false
		}
	}
}

func (c *client) addStar(ctx context.Context) {
	resp, err := c.api.Repository(ctx, &github_client.RepositoryInputArgs{
		Owner: sprRepoOwner,
		Name:  sprRepoName,
	})
	check(err)

	_, err = c.api.AddStar(ctx, &github_client.AddStarInputArgs{
		Input: github_client.AddStarInput{
			StarrableId: resp.Repository.Id,
		},
	})
	check(err)
}
