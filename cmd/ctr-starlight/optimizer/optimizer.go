/*
   file created by Junlin Chen in 2022

*/

package optimizer

import (
	"context"
	"fmt"
	pb "github.com/mc256/starlight/client/api"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

func optimizer(client pb.DaemonClient, req *pb.OptimizeRequest, quiet bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	resp, err := client.SetOptimizer(ctx, req)
	if err != nil {
		fmt.Printf("set optimizer status failed: %v\n", err)
		return
	}
	if resp.Success {
		if !quiet {
			fmt.Printf("set optimizer: %s\n", resp.Message)
		}
	} else {
		fmt.Printf("set optimizer status failed: %s\n", resp.Message)
	}
}

func Action(ctx context.Context, c *cli.Context) (err error) {
	if c.NArg() != 1 {
		return fmt.Errorf("wrong number of arguments, expected 1, got %d", c.NArg())
	}
	action := c.Args().First()
	var a bool
	switch action {
	case "on":
		a = true
		break
	case "off":
		a = false
		break
	default:
		return fmt.Errorf("operation should be 'on' or 'off'")
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

	// set optimizer
	optimizer(pb.NewDaemonClient(conn), &pb.OptimizeRequest{
		Enable: a,
		Group:  c.String("group"),
	}, c.Bool("quiet"))

	return nil
}

func Command() *cli.Command {
	ctx := context.Background()
	return &cli.Command{
		Name: "optimize",
		Usage: `collect filesystem traces to optimize the image.
To collect traces, turn on the optimizer before 'ctr container create' command`,
		Action: func(c *cli.Context) error {
			return Action(ctx, c)
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "group",
				Aliases: []string{"g"},
				Usage:   "group name for collecting multiple traces from multiple containers",
				Value:   "",
			},

			&cli.StringFlag{
				Name:    "quiet",
				Aliases: []string{"q"},
				Value:   "",
				Usage:   "do not print any message unless error occurs",
			},
		},
		ArgsUsage: "on|off",
	}
}
