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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runtime-spec/specs-go"
	"strings"
)

var (
	defaultUnixEnv = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}
)

// replaceOrAppendEnvValues returns the defaults with the overrides either
// replaced by env key or appended to the list
func replaceOrAppendEnvValues(defaults, overrides []string) []string {
	cache := make(map[string]int, len(defaults))
	results := make([]string, 0, len(defaults))
	for i, e := range defaults {
		parts := strings.SplitN(e, "=", 2)
		results = append(results, e)
		cache[parts[0]] = i
	}

	for _, value := range overrides {
		// Values w/o = means they want this env to be removed/unset.
		if !strings.Contains(value, "=") {
			if i, exists := cache[value]; exists {
				results[i] = "" // Used to indicate it should be removed
			}
			continue
		}

		// Just do a normal set/update
		parts := strings.SplitN(value, "=", 2)
		if i, exists := cache[parts[0]]; exists {
			results[i] = value
		} else {
			results = append(results, value)
		}
	}

	// Now remove all entries that we want to "unset"
	for i := 0; i < len(results); i++ {
		if results[i] == "" {
			results = append(results[:i], results[i+1:]...)
			i--
		}
	}

	return results
}

// Copy from spec_opts
func WithImageConfig(cfg []byte) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *oci.Spec) error {
		var (
			ociimage v1.Image
			config   v1.ImageConfig
		)

		if err := json.Unmarshal(cfg, &ociimage); err != nil {
			return err
		}
		config = ociimage.Config

		if s.Process == nil {
			s.Process = &specs.Process{}
		}
		if s.Linux != nil {
			defaults := config.Env
			if len(defaults) == 0 {
				defaults = defaultUnixEnv
			}
			s.Process.Env = replaceOrAppendEnvValues(defaults, s.Process.Env)
			cmd := config.Cmd
			s.Process.Args = append(config.Entrypoint, cmd...)

			cwd := config.WorkingDir
			if cwd == "" {
				cwd = "/"
			}
			s.Process.Cwd = cwd
			if config.User != "" {
				if err := oci.WithUser(config.User)(ctx, client, c, s); err != nil {
					return err
				}
				return oci.WithAdditionalGIDs(fmt.Sprintf("%d", s.Process.User.UID))(ctx, client, c, s)
			}
			// we should query the image's /etc/group for additional GIDs
			// even if there is no specified user in the image config
			return oci.WithAdditionalGIDs("root")(ctx, client, c, s)
		} else if s.Windows != nil {
			s.Process.Env = replaceOrAppendEnvValues(config.Env, s.Process.Env)
			cmd := config.Cmd

			s.Process.Args = append(config.Entrypoint, cmd...)

			s.Process.Cwd = config.WorkingDir
			s.Process.User = specs.User{
				Username: config.User,
			}
		} else {
			return errors.New("spec does not contain Linux or Windows section")
		}
		return nil
	}
}

func validNamespace(ns string) bool {
	linuxNs := specs.LinuxNamespaceType(ns)
	switch linuxNs {
	case specs.PIDNamespace,
		specs.NetworkNamespace,
		specs.UTSNamespace,
		specs.MountNamespace,
		specs.UserNamespace,
		specs.IPCNamespace,
		specs.CgroupNamespace:
		return true
	default:
		return false
	}
}
