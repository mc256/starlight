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
	"github.com/urfave/cli/v2"
)

var (
	Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:     "insecure-source",
			Usage:    "use HTTP for the source image",
			Value:    false,
			Required: false,
		},
		&cli.BoolFlag{
			Name:     "insecure-destination",
			Usage:    "use HTTP for the destination image",
			Value:    false,
			Required: false,
		},
		&cli.StringFlag{
			Name: "platform",
			Usage: "chose what platforms to convert, e.g. 'linux/amd64,linux/arm64', use comma to separate. " +
				"'all' means all platforms in the source image. " +
				"If image is not multi-platform, this flag will be ignored. ",
			Value:    "all",
			Required: false,
		},
	}
)
