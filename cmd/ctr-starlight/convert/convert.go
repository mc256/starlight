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
func Action(ctx context.Context, cli *cli.Context) error {
	// [flags] SourceImage StarlightImage
	if cli.Args().Len() != 2 {
		return errors.New("wrong number of arguments")
	}

	srcImg := cli.Args().Get(0)
	slImg := cli.Args().Get(1)

	srcInsecure := cli.Bool("insecure-source")
	dstInsecure := cli.Bool("insecure-destination")

	// logger
	ns := cli.String("namespace")
	ctx = namespaces.WithNamespace(ctx, ns)
	util.ConfigLoggerWithLevel(ctx, cli.String("log-level"))

	// source
	var srcOptions []name.Option
	if srcInsecure {
		srcOptions = append(srcOptions, name.Insecure)
	}
	var dstOptions []name.Option
	if dstInsecure {
		dstOptions = append(dstOptions, name.Insecure)
	}

	// auth
	remoteOptions := []remote.Option{remote.WithAuthFromKeychain(authn.DefaultKeychain)}

	// config
	convertor, err := util.NewConvertor(ctx, srcImg, slImg, srcOptions, dstOptions, remoteOptions, cli.String("platform"))
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
	if cli.Bool("notify") {
		err = notify.SharedAction(ctx, cli, convertor.GetDst())
		if err != nil {
			log.G(ctx).WithError(err).Error("fail to notify the converted image")
			return nil
		}
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
