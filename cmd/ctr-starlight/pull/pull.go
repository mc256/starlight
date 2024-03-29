/*
   file created by Junlin Chen in 2022

*/

package pull

import (
	"context"
	"fmt"
	"time"

	pb "github.com/mc256/starlight/client/api"
	"github.com/mc256/starlight/cmd/ctr-starlight/auth"
	"github.com/urfave/cli/v2"
	"github.com/vbauerster/mpb/v8/decor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func pullImage(client pb.DaemonClient, ref *pb.ImageReference, quiet bool) error {
	if ref.DisableEarlyStart {
		fmt.Printf("early start has been disabled, this may take a while\n")
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*30)
	defer cancel()
	resp, err := client.PullImage(ctx, ref)
	if err != nil {
		return fmt.Errorf("pull image failed: %v", err)
	}
	if resp.Success {
		if quiet {
			return nil
		}
		end := time.Now()

		if resp.GetMessage() == "ok" {
			if resp.GetBaseImage() == "" {
				fmt.Printf("requested to pull image %s in %dms \n",
					ref.Reference,
					end.Sub(start).Milliseconds(),
				)
			} else {
				fmt.Printf("requested to pull image %s based on %s in %dms \n",
					ref.Reference, resp.GetBaseImage(),
					end.Sub(start).Milliseconds(),
				)
			}
		} else {
			fmt.Printf("%s\n", resp.GetMessage())
		}

		if resp.TotalImageSize > -1 {
			// https://github.com/containers/image/blob/1dca20b9a8417764c0ce9bd4680ce102259ea2ca/copy/progress_bars.go#L24
			// This only includes the HTTP response body, not the HTTP headers.
			skipSize := resp.OriginalImageSize - resp.TotalImageSize
			fmt.Printf("delta image %.1f / original %.1f (skipped: %.1f = %.2f%%)\n",
				decor.SizeB1024(resp.TotalImageSize),
				decor.SizeB1024(resp.OriginalImageSize),
				decor.SizeB1024(skipSize),
				float64(skipSize)/float64(resp.OriginalImageSize)*100,
			)
		}
	} else {
		fmt.Printf("pull image failed: %s\n", resp.Message)
	}
	return nil
}

func Action(ctx context.Context, c *cli.Context) error {
	var base, ref string
	if c.NArg() == 1 {
		ref = c.Args().Get(0)
	} else if c.NArg() == 2 {
		base = c.Args().Get(0)
		ref = c.Args().Get(1)
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
	return pullImage(pb.NewDaemonClient(conn), &pb.ImageReference{
		Reference:         ref,
		Base:              base,
		ProxyConfig:       c.String("profile"),
		Namespace:         c.String("namespace"),
		DisableEarlyStart: c.Bool("disable-early-start"),
	}, c.Bool("quiet"))
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
			&cli.BoolFlag{
				Name:    "disable-early-start",
				Aliases: []string{"w"},
				Value:   false,
				Usage:   "block until the entire image is pulled to the local filesystem",
			},
		),
		ArgsUsage: "[flags] [BaseImage] PullImage",
	}
}
