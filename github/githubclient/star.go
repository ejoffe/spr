package githubclient

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ejoffe/rake"
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/config/config_parser"
	"github.com/ejoffe/spr/github/githubclient/gen/genclient"
	"github.com/rs/zerolog/log"
)

const (
	sprRepoOwner = "ejoffe"
	sprRepoName  = "spr"
	promptCycle  = 25
)

func (c *client) MaybeStar(ctx context.Context, cfg *config.Config) {
	if !cfg.State.Stargazer && cfg.State.RunCount%promptCycle == 0 {
		if c.isStar(ctx) {
			log.Debug().Bool("stargazer", true).Msg("MaybeStar")
			cfg.State.Stargazer = true
			rake.LoadSources(cfg.State,
				rake.YamlFileWriter(config_parser.InternalConfigFilePath()))
		} else {
			log.Debug().Bool("stargazer", false).Msg("MaybeStar")
			fmt.Print("enjoying git spr? add a GitHub star? [Y/n]:")
			reader := bufio.NewReader(os.Stdin)
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)
			if line != "n" {
				log.Debug().Msg("MaybeStar : adding star")
				c.addStar(ctx)
				cfg.State.Stargazer = true
				rake.LoadSources(cfg.State,
					rake.YamlFileWriter(config_parser.InternalConfigFilePath()))
				fmt.Println("Thank you! Happy Coding!")
			}
		}
	}
}

func (c *client) isStar(ctx context.Context) bool {
	iteration := 0
	cursor := ""
	for {
		resp, err := c.api.StarCheck(ctx, &cursor)
		check(err)

		edgeCount := len(*resp.Viewer.StarredRepositories.Edges)
		if edgeCount == 0 {
			log.Debug().Bool("stargazer", false).Msg("MaybeStar::isStar")
			return false
		}

		sprRepo := fmt.Sprintf("%s/%s", sprRepoOwner, sprRepoName)
		for _, node := range *resp.Viewer.StarredRepositories.Nodes {
			if node.NameWithOwner == sprRepo {
				log.Debug().Bool("stargazer", true).Msg("MaybeStar::isStar")
				return true
			}
		}

		edges := *resp.Viewer.StarredRepositories.Edges
		cursor = edges[edgeCount-1].Cursor

		iteration++
		if iteration > 10 {
			// too many stars in the sky
			log.Debug().Bool("stargazer", false).Msg("MaybeStar::isStar (too many stars)")
			return false
		}
	}
}

func (c *client) addStar(ctx context.Context) {
	resp, err := c.api.StarGetRepo(ctx, sprRepoOwner, sprRepoName)
	check(err)

	_, err = c.api.StarAdd(ctx, genclient.AddStarInput{
		StarrableId: resp.Repository.Id,
	})
	check(err)
}
