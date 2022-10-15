/*
   file created by Junlin Chen in 2022

*/

package notify

import (
	"github.com/mc256/starlight/cmd/ctr-starlight/auth"
	"github.com/urfave/cli/v2"
)

var (
	Flags = append([]cli.Flag{
		&cli.BoolFlag{
			Name:     "insecure",
			Usage:    "use HTTP for the registry image",
			Value:    false,
			Required: false,
		},
	}, auth.Flags...)
)
