/*
   file created by Junlin Chen in 2022

*/

package pull

import (
	"context"
	"fmt"
	"github.com/containerd/containerd/log"
	starlight "github.com/mc256/starlight/client/api/v0.2"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

func pullImage(client starlight.ClientAPIClient, ref *starlight.ImageReference) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := client.PullImage(ctx, ref)
	if err != nil {
		log.G(ctx).WithError(err).Error("pull image failed")
		return
	}
	log.G(ctx).Infof("pull image success: %v", resp)
}

func Action(ctx context.Context, c *cli.Context) error {
	if c.NArg() != 1 {
		return fmt.Errorf("no image name provided")
	}

	ref := c.Args().First()
	conn, err := grpc.Dial("localhost:8899", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := starlight.NewClientAPIClient(conn)
	pullImage(client, &starlight.ImageReference{
		Reference: ref,
	})
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
