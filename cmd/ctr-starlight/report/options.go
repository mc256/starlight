/*
   file created by Junlin Chen in 2022

*/

package report

import (
	"github.com/mc256/starlight/cmd/ctr-starlight/auth"
	"github.com/urfave/cli/v2"
)

var (
	Flags = append([]cli.Flag{
		&cli.StringFlag{
			Name:        "path",
			Usage:       "path the the optimizer logs",
			Value:       "/tmp",
			DefaultText: "/tmp",
			Required:    false,
		},
	}, auth.CLIFlags...)
)
