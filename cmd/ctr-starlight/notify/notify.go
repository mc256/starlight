/*
   file created by Junlin Chen in 2022

*/

package notify

import (
	"context"
	"errors"
	"fmt"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/google/go-containerregistry/pkg/name"
	pb "github.com/mc256/starlight/client/api"
	"github.com/mc256/starlight/cmd/ctr-starlight/auth"
	"github.com/mc256/starlight/util"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func notify(ctx context.Context, client pb.DaemonClient, req *pb.NotifyRequest, quiet bool) error {
	resp, err := client.NotifyProxy(context.Background(), req)
	if err != nil {
		return fmt.Errorf("notify starlight proxy server failed: %v", err)
	}
	if resp.Success {
		if !quiet {
			//("notify starlight proxy server success: converted %s\n", resp.GetMessage())
			log.G(ctx).
				WithField("reference", resp.GetMessage()).
				Infof("notify starlight proxy server success")
		}
	} else {
		return errors.New(resp.GetMessage())
	}
	return nil
}

func SharedAction(ctx context.Context, c *cli.Context, reference name.Reference) (err error) {
	// Dial to the daemon
	address := c.String("address")
	opts := grpc.WithTransportCredentials(insecure.NewCredentials())
	conn, err := grpc.Dial(address, opts)
	if err != nil {
		return fmt.Errorf("failed to connect starlight daemon: %v", err)
	}
	defer conn.Close()

	// notify
	return notify(ctx, pb.NewDaemonClient(conn), &pb.NotifyRequest{
		ProxyConfig: c.String("profile"),
		Insecure:    c.Bool("insecure") || c.Bool("insecure-destination"),
		Reference:   reference.String(),
	}, c.Bool("quiet"))
}

func Action(ctx context.Context, c *cli.Context) (err error) {
	// logger
	ns := c.String("namespace")
	util.ConfigLoggerWithLevel(c.String("log-level"))
	ctx = namespaces.WithNamespace(ctx, ns)

	// Parse the reference
	options := []name.Option{}

	if c.NArg() != 1 {
		return errors.New("wrong number of arguments, expected container image reference")
	}

	argRef := c.Args().Get(0)
	if argRef == "" {
		return errors.New("no image reference provided")
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
