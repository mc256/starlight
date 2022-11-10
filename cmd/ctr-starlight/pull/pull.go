/*
   file created by Junlin Chen in 2022

*/

package pull

import (
	"context"
	"fmt"
	pb "github.com/mc256/starlight/client/api"
	"github.com/mc256/starlight/cmd/ctr-starlight/auth"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

func pullImage(client pb.DaemonClient, ref *pb.ImageReference, quiet bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	resp, err := client.PullImage(ctx, ref)
	if err != nil {
		fmt.Printf("pull image failed: %v\n", err)
		return
	}
	if resp.Success {
		if !quiet {
			fmt.Printf("pulling image: %s\n", resp.Message)
		}
	} else {
		fmt.Printf("pull image failed: %s\n", resp.Message)
	}
}

func Action(ctx context.Context, c *cli.Context) error {
	var base, ref string
	if c.NArg() == 1 {
		ref = c.Args().Get(0)
	} else if c.NArg() == 2 {
		ref = c.Args().Get(0)
		base = c.Args().Get(1)
	} else {
		return fmt.Errorf("wrong number of arguments, expected 1 or 2, got %d", c.NArg())
	}

	// Dial to the daemon
	address := c.String("address")
	opts := grpc.WithTransportCredentials(insecure.NewCredentials())
	conn, err := grpc.Dial(address, opts)
	if err != nil {
		fmt.Printf("connect to starlight daemon failed: %v\n", err)
		return nil
	}
	defer conn.Close()

	// pull image
	pullImage(pb.NewDaemonClient(conn), &pb.ImageReference{
		Reference:   ref,
		Base:        base,
		ProxyConfig: c.String("profile"),
	}, c.Bool("quiet"))
	return nil
}

func Command() *cli.Command {
	ctx := context.Background()
	return &cli.Command{
		Name: "pull",
		Usage: "pull image from starlight proxy server, if the base image is not provided, it will choose the latest" +
			" available image with the same name from the same starlight proxy",
		Action: func(c *cli.Context) error {
			return Action(ctx, c)
		},
		Flags: append(
			auth.ProxyFlags,
		),
		ArgsUsage: "[flags] [BaseImage] PullImage",
	}
}
