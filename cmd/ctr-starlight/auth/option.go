/*
   file created by Junlin Chen in 2022

*/

package auth

import "github.com/urfave/cli/v2"

var (
	Flags = []cli.Flag{
		&cli.StringFlag{
			Name:     "server",
			Aliases:  []string{"starlight-proxy"},
			Value:    "starlight.yuri.moe", // public starlight proxy server - for testing only
			Usage:    "the starlight proxy address, report the converted image to the Starlight proxy server",
			Required: false,
			EnvVars:  []string{"STARLIGHT_PROXY"},
		},
		&cli.BoolFlag{
			Name:     "plain-http",
			Aliases:  []string{"insecure-proxy"},
			Usage:    "use plain http connects to the remote server",
			Required: false,
		},

		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Usage: "username for Starlight Proxy authentication. " +
				"(Starlight proxy does not provide authentication, please use a reverse proxy for authentication)",
			EnvVars: []string{"STARLIGHT_USERNAME"},
		},
		&cli.StringFlag{
			Name:    "password",
			Aliases: []string{"p"},
			Usage:   "password for Starlight Proxy authentication",
			EnvVars: []string{"STARLIGHT_PASSWORD"},
		},
	}
)
