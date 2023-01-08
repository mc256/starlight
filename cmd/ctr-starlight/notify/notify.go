/*
   file created by Junlin Chen in 2022

*/

package notify

import (
	"context"
	"errors"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/name"
	pb "github.com/mc256/starlight/client/api"
	"github.com/mc256/starlight/cmd/ctr-starlight/auth"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func notify(client pb.DaemonClient, req *pb.NotifyRequest, quiet bool) {
	resp, err := client.NotifyProxy(context.Background(), req)
	if err != nil {
		fmt.Printf("notify starlight proxy server failed: %v\n", err)
		return
	}
	if resp.Success {
		if !quiet {
			fmt.Printf("notify starlight proxy server success: converted %s\n", resp.GetMessage())
		}
	} else {
		fmt.Printf("notify starlight proxy server failed: %v\n", resp)
	}
}

func SharedAction(ctx context.Context, c *cli.Context, reference name.Reference) (err error) {
	// Dial to the daemon
	address := c.String("address")
	opts := grpc.WithTransportCredentials(insecure.NewCredentials())
	conn, err := grpc.Dial(address, opts)
	if err != nil {
		fmt.Printf("connect to starlight daemon failed: %v\n", err)
		return nil
	}
	defer conn.Close()

	// notify
	notify(pb.NewDaemonClient(conn), &pb.NotifyRequest{
		ProxyConfig: c.String("profile"),
		Insecure:    c.Bool("insecure") || c.Bool("insecure-destination"),
		Reference:   reference.String(),
	}, c.Bool("quiet"))

	return nil
}

func Action(ctx context.Context, c *cli.Context) (err error) {
	options := []name.Option{}

	if c.NArg() != 1 {
		return errors.New("wrong number of arguments, expected container image reference")
	}

	argRef := c.Args().Get(0)
	if argRef == "" {
		log.G(ctx).Fatal("no image reference provided")
		return nil
	}

	reference, err := name.ParseReference(argRef, options...)
	return SharedAction(ctx, c, reference)
}

func Command() *cli.Command {
	ctx := context.Background()
	cmd := cli.Command{
		Name:  "notify",
		Usage: "Notify the Starlight Proxy that a new Starlight image is available",
		Action: func(c *cli.Context) error {
			return Action(ctx, c)
		},
		Flags: append(auth.ProxyFlags,
			&cli.BoolFlag{
				Name:     "insecure",
				Usage:    "use HTTP registry",
				Value:    false,
				Required: false,
			}),
		ArgsUsage: "[flags] SourceImage StarlightImage",
	}
	return &cmd
}
