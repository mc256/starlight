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

package create

import (
	"github.com/urfave/cli/v2"
)

var (
	StarlightFlags = []cli.Flag{
		&cli.IntFlag{
			Name:     "start-checkpoint",
			Aliases:  []string{"cp"},
			Usage:    "wait until a specific checkpoint  (0 will start the container immediately and likely block on IO)",
			Value:    0,
			Required: false,
		},
		&cli.BoolFlag{
			Name:     "optimize",
			Usage:    "collect the traces of the file access for optimization",
			Value:    false,
			Required: false,
		},
	}

	// ContainerFlags are cli flags specifying container options
	ContainerFlags = []cli.Flag{
		&cli.StringFlag{
			Name:  "cwd",
			Usage: "specify the working directory of the process",
		},
		&cli.StringSliceFlag{
			Name:  "env",
			Usage: "specify additional container environment variables (i.e. FOO=bar)",
		},
		&cli.StringFlag{
			Name:  "env-file",
			Usage: "specify additional container environment variables in a file(i.e. FOO=bar, one per line)",
		},
		&cli.StringSliceFlag{
			Name:  "label",
			Usage: "specify additional labels (i.e. foo=bar)",
		},
		&cli.StringSliceFlag{
			Name:  "mount",
			Usage: "specify additional container mount (ex: type=bind,src=/tmp,dst=/host,options=rbind:ro)",
		},
		&cli.BoolFlag{
			Name:  "net-host",
			Usage: "enable host networking for the container",
		},
		&cli.BoolFlag{
			Name:  "privileged",
			Usage: "run privileged container",
		},
		&cli.BoolFlag{
			Name:  "tty,t",
			Usage: "allocate a TTY for the container",
		},
		&cli.StringSliceFlag{
			Name:  "with-ns",
			Usage: "specify existing Linux namespaces to join at container runtime (format '<nstype>:<path>')",
		},
		&cli.IntFlag{
			Name:  "gpus",
			Usage: "add gpus to the container",
		},
		&cli.Uint64Flag{
			Name:  "memory-limit",
			Usage: "memory limit (in bytes) for the container",
		},
		&cli.StringSliceFlag{
			Name:  "device",
			Usage: "add a device to a container",
		},
		&cli.BoolFlag{
			Name:  "local-time",
			Usage: "synchronize host local time",
		},
		&cli.StringFlag{
			Name:  "host-name",
			Usage: "host name for this container worker",
		},
	}
)
