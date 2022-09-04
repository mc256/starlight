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
	"github.com/containerd/containerd/platforms"
	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mc256/starlight/proxy"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func Action(c *cli.Context) error {
	// [flags] SourceImage StarlightImage
	if c.Args().Len() != 2 {
		return errors.New("wrong number of arguments")
	}

	srcImg := c.Args().Get(0)
	slImg := c.Args().Get(1)

	srcInsecure := c.Bool("insecure-source")
	dstInsecure := c.Bool("insecure-destination")
	platform := c.String("platform")

	// logger
	ns := c.String("namespace")
	util.ConfigLoggerWithLevel(c.String("log-level"))
	ctx := namespaces.WithNamespace(context.Background(), ns)

	// source
	srcOptions := []name.Option{}
	if srcInsecure {
		srcOptions = append(srcOptions, name.Insecure)
	}
	dstOptions := []name.Option{}
	if dstInsecure {
		dstOptions = append(dstOptions, name.Insecure)
	}

	// platform
	remoteOptions := []remote.Option{remote.WithAuthFromKeychain(authn.DefaultKeychain)}
	if platform != "" {
		if p, err := platforms.Parse(platform); err != nil {
			remoteOptions = append(remoteOptions, remote.WithPlatform(v1.Platform{
				Architecture: p.Architecture,
				OS:           p.OS,
				OSVersion:    p.OSVersion,
				OSFeatures:   p.OSFeatures,
				Variant:      p.Variant,
			}))
		}
	}

	// config
	convertor, err := proxy.NewConvertor(ctx, srcImg, slImg, srcOptions, dstOptions, remoteOptions)
	if err != nil {
		log.G(ctx).WithError(err).Error("illegal image reference")
		return nil
	}
	log.G(ctx).WithFields(logrus.Fields{
		"from": convertor.GetSrc(),
		"to":   convertor.GetDst(),
	}).Info("convert container image to Starlight format")

	// Convert
	err = convertor.ToStarlightImage()
	if err != nil {
		log.G(ctx).WithError(err).Error("fail to convert the container image")
		return nil
	}
	log.G(ctx).Info("conversion completed")

	return nil
}

func Command() *cli.Command {
	cmd := cli.Command{
		Name:  "convert",
		Usage: "Convert typical container image (in .tar.gz or .tar format) to Starlight image format",
		Action: func(c *cli.Context) error {
			return Action(c)
		},
		Flags:     append(RegistryFlags),
		ArgsUsage: "[flags] SourceImage StarlightImage",
	}
	return &cmd
}
