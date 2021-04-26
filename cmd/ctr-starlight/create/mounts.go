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
	gocontext "context"
	"encoding/csv"
	"fmt"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"os"
	"path"
	"strings"
	"time"
)

func touchFile(ctx gocontext.Context, mounting, fileName string) error {
	fullName := path.Join(mounting, fileName)
	log.G(ctx).WithFields(logrus.Fields{
		"path": fullName,
	}).Debug("preparing guest binding file")

	create := false
	if stat, err := os.Stat(fullName); os.IsNotExist(err) {
		create = true
	} else if stat.IsDir() {
		if err := os.RemoveAll(fullName); err != nil {
			return err
		}
		create = true
	}

	if create {
		if err := os.MkdirAll(path.Dir(fullName), 0755); err != nil {
			return err
		}
		if file, err := os.Create(fullName); err != nil {
			return err
		} else {
			return file.Close()
		}
	} else {
		currentTime := time.Now().Local()
		return os.Chtimes(fullName, currentTime, currentTime)
	}
}

func touchDir(ctx gocontext.Context, mounting, dirName string) error {
	fullName := path.Join(mounting, dirName)
	log.G(ctx).WithFields(logrus.Fields{
		"path": fullName,
	}).Debug("preparing guest binding directory")

	create := false
	if stat, err := os.Stat(fullName); os.IsNotExist(err) {
		create = true
	} else if stat.IsDir() {
		if err := os.RemoveAll(fullName); err != nil {
			return err
		}
		create = true
	}

	if create {
		if err := os.MkdirAll(fullName, 0755); err != nil {
			return err
		}
		return nil
	} else {
		currentTime := time.Now().Local()
		return os.Chtimes(fullName, currentTime, currentTime)
	}
}

func withMounts(context *cli.Context, ctx gocontext.Context, base string) (oci.SpecOpts, error) {
	for _, mount := range context.StringSlice("mount") {
		m, err := parseMountFlag(mount)
		if err != nil {
			return nil, err
		}

		if stat, err := os.Stat(m.Source); err != nil {
			return nil, err
		} else {
			if stat.IsDir() {
				if err := touchDir(ctx, base, m.Destination); err != nil {
					return nil, err
				}
			} else {
				if err := touchFile(ctx, base, m.Destination); err != nil {
					return nil, err
				}
			}
		}
	}

	return func(ctx gocontext.Context, client oci.Client, container *containers.Container, s *specs.Spec) error {
		mounts := make([]specs.Mount, 0)
		for _, mount := range context.StringSlice("mount") {
			m, err := parseMountFlag(mount)
			if err != nil {
				return err
			}

			mounts = append(mounts, m)

			log.G(ctx).WithFields(logrus.Fields{
				"destination": m.Destination,
				"source":      m.Source,
			}).Debug("mounting point")

		}
		return oci.WithMounts(mounts)(ctx, client, container, s)
	}, nil
}

// parseMountFlag parses a mount string in the form "type=foo,source=/path,destination=/target,options=rbind:rw"
func parseMountFlag(m string) (specs.Mount, error) {
	mount := specs.Mount{}
	r := csv.NewReader(strings.NewReader(m))

	fields, err := r.Read()
	if err != nil {
		return mount, err
	}

	for _, field := range fields {
		v := strings.Split(field, "=")
		if len(v) != 2 {
			return mount, fmt.Errorf("invalid mount specification: expected key=val")
		}

		key := v[0]
		val := v[1]
		switch key {
		case "type":
			mount.Type = val
		case "source", "src":
			mount.Source = val
		case "destination", "dst":
			mount.Destination = val
		case "options":
			mount.Options = strings.Split(val, ":")
		default:
			return mount, fmt.Errorf("mount option %q not supported", key)
		}
	}

	return mount, nil
}
