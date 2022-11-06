/*
   file created by Junlin Chen in 2022

*/

package version

import (
	"context"
	"fmt"
	pb "github.com/mc256/starlight/client/api"
	"github.com/mc256/starlight/util"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

func getVersion(client pb.DaemonClient) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	resp, err := client.GetVersion(ctx, &pb.Request{})
	if err != nil {
		return "", err
	}
	return resp.Version, nil
}

func Action(context *cli.Context) error {
	fmt.Printf("ctr-starlight %s\n", util.Version)

	// Dial to the daemon
	address := context.String("address")
	opts := grpc.WithTransportCredentials(insecure.NewCredentials())
	conn, err := grpc.Dial(address, opts)
	if err != nil {
		fmt.Printf("connect to starlight daemon failed: %v\n", err)
		return nil
	}
	defer conn.Close()

	// Get Version
	v, err := getVersion(pb.NewDaemonClient(conn))
	if err != nil {
		fmt.Printf("failed to obtain starlight daemon version: %v\n", err)
		return nil
	}
	fmt.Printf("starlight-daemon %s\n", v)

	return nil
}

func Command() *cli.Command {
	cmd := cli.Command{
		Name:  "version",
		Usage: "Show version information",
		Action: func(context *cli.Context) error {
			return Action(context)
		},
	}
	return &cmd
}
