/*
   file created by Junlin Chen in 2022

*/

package auth

import "github.com/urfave/cli/v2"

var (
	ProxyFlags = []cli.Flag{
		&cli.StringFlag{
			Name:    "profile",
			Aliases: []string{"p"},
			Value:   "",
			Usage: "profile name for connecting to the starlight proxy server in the configuration file, " +
				"if leave empty will use the default profile",
		},
		&cli.StringFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Value:   "",
			Usage:   "do not print any message unless error occurs",
		},
	}
)
