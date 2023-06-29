/*
   Copyright The starlight Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.

   file created by maverick in 2021
*/

package report

import (
	"context"
	"fmt"
	"time"

	pb "github.com/mc256/starlight/client/api"
	"github.com/mc256/starlight/cmd/ctr-starlight/auth"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// report uses the daemon client to report traces, it does not report directly to the proxy
// Because it does not handle the credential and authentication of the proxy.
func report(client pb.DaemonClient, req *pb.ReportTracesRequest, quiet bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	resp, err := client.ReportTraces(ctx, req)
	if err != nil {
		return fmt.Errorf("report traces failed: %v", err)
	}
	if resp.Success {
		if !quiet {
			fmt.Printf("reported traces: %s\n", resp.Message)
		}
	} else {
		fmt.Printf("report traces failed: %s\n", resp.Message)
	}
	return nil
}

func Action(ctx context.Context, c *cli.Context) (err error) {
	// Dial to the daemon
	address := c.String("address")
	opts := grpc.WithTransportCredentials(insecure.NewCredentials())
	conn, err := grpc.Dial(address, opts)
	if err != nil {
		fmt.Printf("connect to starlight daemon failed: %v\n", err)
		return nil
	}
	defer conn.Close()

	// report
	return report(pb.NewDaemonClient(conn), &pb.ReportTracesRequest{
		ProxyConfig: c.String("profile"),
	}, c.Bool("quiet"))
}

func Command() *cli.Command {
	ctx := context.Background()
	cmd := cli.Command{
		Name:  "report",
		Usage: "Report filesystem traces back to Starlight Proxy to speed up other similar deployment",
		Action: func(c *cli.Context) error {
			return Action(ctx, c)
		},
		Flags:     auth.ProxyFlags,
		ArgsUsage: "",
	}
	return &cmd
}
