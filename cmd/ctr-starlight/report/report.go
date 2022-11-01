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
	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/client/fs"
	"github.com/mc256/starlight/proxy"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func Action(ctx context.Context, c *cli.Context) (err error) {
	var tc *fs.TraceCollection
	tc, err = fs.NewTraceCollection(ctx, c.String("path"))
	if err != nil {
		return err
	}
	protocol := "https"
	if c.Bool("plain-http") {
		protocol = "http"
	}

	server := c.String("server")
	if server == "starlight.yuri.moe" {
		protocol = "https"
		log.G(ctx).Warn("using public staging starlight proxy server. " +
			"the public server may not have your own container image, " +
			"please set your own starlight server using environment variable STARLIGHT_PROXY or --server flag")
	}

	if server == "" {
		log.G(ctx).Fatal("no starlight proxy server address provided")
		return nil
	}

	log.G(ctx).WithFields(logrus.Fields{
		"server":   server,
		"protocol": protocol,
	}).Info("uploading data to starlight proxy server")

	p := proxy.NewStarlightProxy(ctx, protocol, c.String("server"))

	/*
		if err = proxy.Report(ref, tc.ToJSONBuffer()); err != nil {
			return err
		}
	*/

	fmt.Println(p, tc)

	return nil
}

func Command() *cli.Command {
	ctx := context.Background()
	cmd := cli.Command{
		Name:  "report",
		Usage: "Upload data collected by the optimizer back to Starlight Proxy to speed up other similar deployment",
		Action: func(c *cli.Context) error {
			return Action(ctx, c)
		},
		Flags:     Flags,
		ArgsUsage: "",
	}
	return &cmd
}
