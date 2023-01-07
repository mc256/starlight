/*
   file created by cstria0106 in 2022

*/

package listproxy

import (
	"context"
	"fmt"
	"time"

	pb "github.com/mc256/starlight/client/api"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// listProxyProfile prints the list of proxy profiles in Starlight daemon configuration
func listProxyProfile(client pb.DaemonClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	resp, err := client.GetProxyProfiles(ctx, &pb.Request{})
	if err != nil {
		return errors.Wrapf(err, "failed to list proxy profiles")
	}

	// get max length of the name
	maxLen := 0
	for _, profile := range resp.Profiles {
		if len(profile.Name) > maxLen {
			maxLen = len(profile.Name)
		}
	}

	// print
	for _, profile := range resp.Profiles {
		fmt.Printf("%*s %s://%s\n", maxLen+2, fmt.Sprintf("[%s]", profile.Name), profile.Protocol, profile.Address)
	}

	return nil
}

func Action(context *cli.Context) (err error) {
	// Dial to the daemon
	address := context.String("address")
	opts := grpc.WithTransportCredentials(insecure.NewCredentials())
	conn, err := grpc.Dial(address, opts)
	if err != nil {
		fmt.Printf("connect to starlight daemon failed: %v\n", err)
		return nil
	}
	defer conn.Close()

	if context.NArg() != 0 {
		return fmt.Errorf("invalid number of arguments")
	}

	// send to proxy server
	err = listProxyProfile(pb.NewDaemonClient(conn))
	if err != nil {
		return err
	}

	return nil
}

func Command() *cli.Command {
	cmd := cli.Command{
		Name:    "list-proxy",
		Aliases: []string{"ls", "lp"},
		Usage:   "list starlight proxy servers in the configuration",
		Action: func(c *cli.Context) error {
			return Action(c)
		},
		ArgsUsage: "",
	}
	return &cmd
}
