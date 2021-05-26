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

package pull

import (
	"errors"
	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/ctr"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func Action(c *cli.Context) error {
	var fromImages, toImages string
	if c.Args().Len() == 1 {
		fromImages = ""
		toImages = c.Args().First()
	} else if c.Args().Len() == 2 {
		fromImages = c.Args().Get(0)
		toImages = c.Args().Get(1)
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
		"from": fromImages,
		"to":   toImages,
	}).Info("preparing delta image")

	// Prepare delta image
	if err = t.Sn.PrepareDeltaImage(fromImages, toImages); err != nil {
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
		Flags:     []cli.Flag{},
		ArgsUsage: "[FromImages] ToImages",
	}
	return &cmd
}
