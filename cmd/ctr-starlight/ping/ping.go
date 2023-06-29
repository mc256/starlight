/*
   file created by Junlin Chen in 2022

*/

package ping

import (
	"context"
	"errors"
	"fmt"

	pb "github.com/mc256/starlight/client/api"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func ping(client pb.DaemonClient, req *pb.PingRequest, quiet bool) {
	resp, err := client.PingTest(context.Background(), req)
	if err != nil {
		fmt.Printf("network test failed: %v\n", err)
		return
	}
	if resp.Success {
		if !quiet {
			fmt.Printf("ping test success: %v\nlatency: %d ms\n", resp.GetMessage(), resp.GetLatency())
		}
	} else {
		fmt.Printf("network test failed: %v\n", resp.GetMessage())
	}
}

func Action(ctx context.Context, c *cli.Context) (err error) {
	if c.NArg() > 1 {
		return errors.New("wrong number of arguments, expected container image reference")
	}

	profile := c.Args().First()

	// Dial to the daemon
	address := c.String("address")
	opts := grpc.WithTransportCredentials(insecure.NewCredentials())
	conn, err := grpc.Dial(address, opts)
	if err != nil {
		fmt.Printf("connect to starlight daemon failed: %v\n", err)
		return nil
	}
	defer conn.Close()

	// ping
	ping(pb.NewDaemonClient(conn), &pb.PingRequest{
		ProxyConfig: profile,
	}, c.Bool("quiet"))

	return nil
}

func Command() *cli.Command {
	ctx := context.Background()
	cmd := cli.Command{
		Name:    "test",
		Aliases: []string{"ping"},
		Usage:   "testing network connection between starlight proxy and starlight daemon, returns HTTP RTT in ms",
		Action: func(c *cli.Context) error {
			return Action(ctx, c)
		},
		ArgsUsage: "[proxy-profile-name]",
	}
	return &cmd
}
