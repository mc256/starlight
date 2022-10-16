/*
   file created by Junlin Chen in 2022

*/

package notify

import (
	"context"
	"errors"
	"github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mc256/starlight/grpc"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

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

func SharedAction(ctx context.Context, c *cli.Context, reference name.Reference) (err error) {
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
	}).Info("notify starlight proxy server")

	proxy := grpc.NewStarlightProxy(ctx, protocol, c.String("server"))
	if err = proxy.Notify(reference); err != nil {
		log.G(ctx).WithError(err).Error("failed to notify starlight proxy server")
		return nil
	}
	return nil
}

func Command() *cli.Command {
	ctx := context.Background()
	cmd := cli.Command{
		Name:  "notify",
		Usage: "Notify the Starlight Proxy that a new Starlight image is available",
		Action: func(c *cli.Context) error {
			return Action(ctx, c)
		},
		Flags: append(
			Flags,
		),
		ArgsUsage: "[flags] SourceImage StarlightImage",
	}
	return &cmd
}
