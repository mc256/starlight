/*
   file created by Junlin Chen in 2022

*/

package addproxy

import (
	"context"
	"fmt"
	pb "github.com/mc256/starlight/client/api"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

func addProxyProfile(client pb.DaemonClient, name, protocol, address, username, password string, quiet bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	resp, err := client.AddProxyProfile(ctx, &pb.AuthRequest{
		ProfileName: name,
		Protocol:    protocol,
		Address:     address,

		Username: username,
		Password: password,
	})
	if err != nil {
		return err
	}
	if resp.Success {
		if !quiet {
			fmt.Printf("proxy profile %s added\n", name)
		}
	} else {
		fmt.Printf("failed to add proxy profile %s: %s\n", name, resp.Message)
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

	// parse args
	var (
		proxyName string
		proxyAddr string
		protocol  string
		username  string
		password  string
	)
	if context.NArg() == 3 {
		proxyName = context.Args().Get(0)
		protocol = context.Args().Get(1)
		proxyAddr = context.Args().Get(2)
	} else if context.NArg() == 4 {
		proxyName = context.Args().Get(0)
		protocol = context.Args().Get(1)
		proxyAddr = context.Args().Get(2)
		username = context.Args().Get(3)
		fmt.Printf("password for %s: ", username)
		_, err = fmt.Scanln(&password)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("invalid number of arguments")
	}

	// send to proxy server
	err = addProxyProfile(pb.NewDaemonClient(conn),
		proxyName,
		protocol, proxyAddr,
		username, password,
		context.Bool("quiet"))
	if err != nil {
		return err
	}

	return nil
}

func Command() *cli.Command {
	cmd := cli.Command{
		Name:    "add-proxy",
		Aliases: []string{"add", "ap"},
		Usage:   "add a starlight proxy server to the configuration",
		Action: func(c *cli.Context) error {
			return Action(c)
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "password",
				Aliases: []string{"p"},
				Value:   "",
				Usage:   "provide username and password for the proxy server",
			},
			&cli.StringFlag{
				Name:    "quiet",
				Aliases: []string{"q"},
				Value:   "",
				Usage:   "do not print any message unless error occurs",
			},
		},
		ArgsUsage: "[flags] proxy-name (http|https) proxy-address [username]",
	}
	return &cmd
}
