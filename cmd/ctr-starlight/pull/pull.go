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
	var from, to string
	if c.Args().Len() == 1 {
		from = ""
		to = c.Args().First()
	} else if c.Args().Len() == 2 {
		from = c.Args().Get(0)
		to = c.Args().Get(1)
	} else {
		return errors.New("wrong arguments")
	}

	ns := c.String("namespace")
	socket := c.String("address")

	// Connect to containerd
	t, ctx, err := ctr.NewContainerdClient(ns, socket, c.String("log-level"))
	if err != nil {
		log.G(ctx).WithError(err).Error("containerd client")
		return nil
	}

	log.G(ctx).WithFields(logrus.Fields{
		"from": from,
		"to":   to,
	}).Info("preparing delta image")

	// Prepare delta image
	if err = t.Sn.PrepareDeltaImage(from, to); err != nil {
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
				Name:     "optimize-group",
				Usage:    "label of this workflow",
				Value:    "",
				Aliases:  []string{"app", "workload"},
				Required: false,
			},
		},
		ArgsUsage: "[FromImages] ToImages",
	}
	return &cmd
}
