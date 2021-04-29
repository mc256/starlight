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
	"errors"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/contrib/nvidia"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/oci"
	"github.com/mc256/starlight/ctr"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"path"
	"strings"
)

const (
	RunCommand = false
)

func Action(c *cli.Context) error {
	// [flags] ImageCombination Image CONTAINER [COMMAND] [ARG...]
	var imageCombo, ref, containerName string
	var args []string
	if c.Args().Len() >= 3 {
		imageCombo = c.Args().Get(0)
		ref = c.Args().Get(1)
		containerName = c.Args().Get(2)
		args = c.Args().Slice()[3:]
	} else {
		return errors.New("wrong number of arguments")
	}

	// Connect to containerd
	ns := c.String("namespace")
	socket := c.String("address")
	t, ctx, err := ctr.NewContainerdClient(ns, socket, c.String("log-level"))
	if err != nil {
		log.G(ctx).WithError(err).Error("containerd client")
		return nil
	}
	log.G(ctx).WithFields(logrus.Fields{
		"combo":     imageCombo,
		"ref":       ref,
		"container": containerName,
	}).Info("preparing snapshot")

	// container image reference
	var name, tag string
	if sp := strings.Split(ref, ":"); len(sp) != 2 {
		log.G(ctx).Error("invalid image name")
		return nil
	} else {
		name = sp[0]
		tag = sp[1]
	}

	// Prepare snapshot
	checkpoint := c.Int("start-checkpoint")
	var mnt []mount.Mount
	if t.SnId, mnt, err = t.Sn.PrepareContainerSnapshot(name, tag, imageCombo, checkpoint); err != nil {
		log.G(ctx).Error(err)
		return nil
	} else {
		log.G(ctx).WithFields(logrus.Fields{
			"mnt":         mnt,
			"snapshotter": t.SnId,
		}).Info("prepared container snapshot")
	}

	// Options - Container Initials
	var (
		opts  []oci.SpecOpts
		cOpts []containerd.NewContainerOpts
		spec  containerd.NewContainerOpts
	)

	cOpts = append(cOpts, containerd.WithContainerLabels(commands.LabelArgs(c.StringSlice("label"))))
	cOpts = append(cOpts,
		containerd.WithSnapshotter("starlight"),
		containerd.WithSnapshot(t.SnId),
		containerd.WithImageName(containerName),
	)

	// Options - OCI specs Initials
	opts = append(opts, oci.WithDefaultSpec(), oci.WithDefaultUnixDevices)
	configPath := path.Join(mnt[0].Source, "../..", fmt.Sprintf("%s_%s.json", name, tag))
	if config, err := ioutil.ReadFile(configPath); err == nil {
		opts = append(opts, WithImageConfig(config))
		log.G(ctx).WithField("path", configPath).Info("added image config")
	} else {
		log.G(ctx).WithError(err).Error("cannot open image config")
	}

	// Options env
	if ef := c.String("env-file"); ef != "" {
		opts = append(opts, oci.WithEnvFile(ef))
	}

	opts = append(opts, oci.WithEnv(c.StringSlice("env")))

	// Options mounts
	ms, err := withMounts(c, ctx, mnt[0].Source)
	if err != nil {
		return err
	}
	opts = append(opts, ms)

	////////////////////////////////////////////////////////////////
	if len(args) > 0 {
		opts = append(opts, oci.WithProcessArgs(args...))
	}
	if cwd := c.String("cwd"); cwd != "" {
		opts = append(opts, oci.WithProcessCwd(cwd))
	}
	if c.Bool("tty") {
		opts = append(opts, oci.WithTTY)
	}

	////////////////////////////////////////////////////////////////
	if c.Bool("local-time") {
		opts = append(opts, oci.WithHostLocaltime)
		if err := touchFile(ctx, mnt[0].Source, "etc/localtime"); err != nil {
			log.G(ctx).WithError(err).Error("touch localtime error")
		}
	}
	if c.Bool("privileged") {
		opts = append(opts, oci.WithPrivileged, oci.WithAllDevicesAllowed, oci.WithHostDevices)
	}
	if c.Bool("net-host") {
		opts = append(opts, oci.WithHostNamespace(specs.NetworkNamespace), oci.WithHostHostsFile, oci.WithHostResolvconf)
		if err := touchFile(ctx, mnt[0].Source, "etc/hosts"); err != nil {
			log.G(ctx).WithError(err).Error("touch hosts error")
		}
		if err := touchFile(ctx, mnt[0].Source, "etc/resolv.conf"); err != nil {
			log.G(ctx).WithError(err).Error("touch resolv.conf error")
		}
	}

	////////////////////////////////////////////////////////////////
	if cpus := c.Float64("cpus"); cpus > 0.0 {
		var (
			period = uint64(100000)
			quota  = int64(cpus * 100000.0)
		)
		opts = append(opts, oci.WithCPUCFS(quota, period))
	}

	quota := c.Int64("cpu-quota")
	period := c.Uint64("cpu-period")
	if quota != -1 || period != 0 {
		if cpus := c.Float64("cpus"); cpus > 0.0 {
			err := errors.New("cpus and quota/period should be used separately")
			log.G(ctx).WithError(err).Error("parameter error: cpu-quota")
			return nil
		}
		opts = append(opts, oci.WithCPUCFS(quota, period))
	}

	joinNs := c.StringSlice("with-ns")
	for _, ns := range joinNs {
		parts := strings.Split(ns, ":")
		if len(parts) != 2 {
			err := errors.New("joining a Linux namespace using --with-ns requires the format 'nstype:path'")
			log.G(ctx).WithError(err).Error("parameter error: with-ns")
			return nil

		}
		if !validNamespace(parts[0]) {
			err := errors.New("the Linux namespace type specified in --with-ns is not valid: " + parts[0])
			log.G(ctx).WithError(err).Error("parameter error: with-ns")
			return nil

		}
		opts = append(opts, oci.WithLinuxNamespace(specs.LinuxNamespace{
			Type: specs.LinuxNamespaceType(parts[0]),
			Path: parts[1],
		}))
	}
	if c.IsSet("gpus") {
		opts = append(opts, nvidia.WithGPUs(nvidia.WithDevices(c.Int("gpus")), nvidia.WithAllCapabilities))
	}

	if c.IsSet("cgroup") {
		// NOTE: can be set to "" explicitly for disabling cgroup.
		opts = append(opts, oci.WithCgroup(c.String("cgroup")))
	}
	limit := c.Uint64("memory-limit")
	if limit != 0 {
		opts = append(opts, oci.WithMemoryLimit(limit))
	}
	for _, dev := range c.StringSlice("device") {
		opts = append(opts, oci.WithLinuxDevice(dev, "rwm"))
	}

	cOpts = append(cOpts, WithImageStopSignal())

	////////////////////////////////////////////////////////////////
	opts = append(opts, oci.WithAnnotations(commands.LabelArgs(c.StringSlice("label"))))
	var s specs.Spec
	spec = containerd.WithSpec(&s, opts...)

	cOpts = append(cOpts, spec)

	// Run Container
	if t.Container, err = t.Client.NewContainer(ctx, containerName, cOpts...); err != nil {
		log.G(ctx).Error(err)
		return err
	}
	log.G(ctx).Info("container created")

	return nil
}

func Command() *cli.Command {
	cmd := cli.Command{
		Name:  "create",
		Usage: "Create container",
		Action: func(c *cli.Context) error {
			return Action(c)
		},
		Flags:     append(ContainerFlags, StarlightFlags...),
		ArgsUsage: "[flags] ImageCombination Image CONTAINER [COMMAND] [ARG...]",
	}
	return &cmd
}
