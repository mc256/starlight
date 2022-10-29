/*
   file created by Junlin Chen in 2022

*/

package pull

import (
	"context"
	"github.com/urfave/cli/v2"
)

func Action(ctx context.Context, c *cli.Context) error {
	return nil
}

func Command() *cli.Command {
	ctx := context.Background()
	return &cli.Command{
		Name:  "pull",
		Usage: "pull image from starlight proxy server",
		Action: func(c *cli.Context) error {
			return Action(ctx, c)
		},
	}
}
