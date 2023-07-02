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

   file created by maverick in 2022
*/

package convert

import (
	"context"
	"errors"
	"fmt"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/mc256/starlight/cmd/ctr-starlight/auth"
	"github.com/mc256/starlight/cmd/ctr-starlight/notify"
	"github.com/mc256/starlight/util"
	"github.com/urfave/cli/v2"
)

// Action - This Action does not require communicates to the Starlight daemon.
func Action(ctx context.Context, c *cli.Context) error {
	// [flags] SourceImage StarlightImage
	if c.Args().Len() != 2 {
		return errors.New("wrong number of arguments")
	}

	srcImg := c.Args().Get(0)
	slImg := c.Args().Get(1)

	srcInsecure := c.Bool("insecure-source")
	dstInsecure := c.Bool("insecure-destination")

	// logger
	ns := c.String("namespace")
	util.ConfigLoggerWithLevel(c.String("log-level"))
	ctx = namespaces.WithNamespace(ctx, ns)

	// source
	srcOptions := []name.Option{}
	if srcInsecure {
		srcOptions = append(srcOptions, name.Insecure)
	}
	dstOptions := []name.Option{}
	if dstInsecure {
		dstOptions = append(dstOptions, name.Insecure)
	}

	// auth
	remoteOptions := []remote.Option{remote.WithAuthFromKeychain(authn.DefaultKeychain)}

	// config
	convertor, err := util.NewConvertor(ctx, srcImg, slImg, srcOptions, dstOptions, remoteOptions, c.String("platform"))
	if err != nil {
		log.G(ctx).WithError(err).Error("illegal image reference")
		return nil
	}

	// convert
	err = convertor.ToStarlightImage()
	if err != nil {
		log.G(ctx).WithError(err).Error("fail to convert the container image")
		return nil
	}
	log.G(ctx).Info("conversion completed")

	// notify
	if c.Bool("notify") {
		err = notify.SharedAction(ctx, c, convertor.GetDst())
		if err != nil {
			log.G(ctx).WithError(err).Error("fail to notify the converted image")
			flags := " "
			if dstInsecure {
				flags += "--insecure "
			}
			if c.String("profile") != "" {
				flags += fmt.Sprintf("--profile=%s ", c.String("profile"))
			}
			return fmt.Errorf("fail to notify the converted image, try again using `ctr-starlight%snotify %s`", flags, slImg)
		}
		log.G(ctx).Info("notified starlight proxy")
	}

	return nil
}

func Command() *cli.Command {
	ctx := context.Background()
	cmd := cli.Command{
		Name: "convert",
		Usage: "Convert typical container image (in .tar.gz or .tar format) to Starlight image format. " +
			"Credentials for private registry can be configured in $DOCKER_CONFIG.",
		Action: func(c *cli.Context) error {
			return Action(ctx, c)
		},
		Flags: append(

			// Convert Flags
			Flags,

			// Report Flags
			append(
				auth.ProxyFlags,
				&cli.BoolFlag{
					Name:     "notify",
					Usage:    "notify the converted image to the Starlight Proxy",
					Value:    false,
					Required: false,
				},
			)...,
		),
		ArgsUsage: "[flags] SourceImage StarlightImage",
	}
	return &cmd
}
