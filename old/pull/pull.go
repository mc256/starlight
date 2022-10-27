/*
   file created by Junlin Chen in 2022

*/

package pull

import (
	"errors"
	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/ctr"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func Action(c *cli.Context) error {
	var ref string
	if c.Args().Len() == 1 {
		ref = c.Args().First()
	} else {
		return errors.New("wrong arguments")
	}

	ns := c.String("namespace")
	socket := c.String("address")
	from := c.String("from")
	proxy := c.String("proxy")

	// Connect to containerd
	t, ctx, err := ctr.NewContainerdClient(ns, socket, c.String("log-level"))
	if err != nil {
		log.G(ctx).WithError(err).Error("containerd client")
		return nil
	}

	// log
	log.G(ctx).WithFields(logrus.Fields{
		"ref":   ref,
		"proxy": proxy,
		"from":  from,
	}).Info("preparing delta image")

	// Prepare delta image
	if err = t.Sn.Pull(from, proxy, ref); err != nil {
		log.G(ctx).WithError(err).Error("prepare delta image")
		return nil
	}
	log.G(ctx).Info("prepared delta image")

	return nil
}

func Command() *cli.Command {
	cmd := cli.Command{
		Name:  "pull",
		Usage: "Launch background fetcher to load the delta image",
		Action: func(c *cli.Context) error {
			return Action(c)
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "from",
				Usage:    "specify a particular container image that the (if not specified, the latest downloaded container image with the same 'image name' will be used)",
				Value:    "",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "proxy",
				Usage:    "override the default Starlight Proxy address if provided",
				Value:    "",
				Required: false,
			},
			// Pull Group
		},
		ArgsUsage: "Image",
	}
	return &cmd
}
