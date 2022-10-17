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

package ctr

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/mc256/starlight/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"math/rand"
	"time"
)

/*
	 prepare: target() [name] <------------------------|
			 |                                         |
	  commit: accelerated()  [key] -------------------|
			 |
	  prepare: worker-N  []{mountingPoint...}
			 |
		mount: [mountingPoint]
			 |
	  commit: committed-worker-N
*/

type snOptFunc func() snapshots.Opt

type SnapshotterService struct {
	ctx context.Context
	sn  snapshots.Snapshotter
	id  string
}

func getAcceleratedSnapshotterName(ref string) string {
	return fmt.Sprintf("accerated-%s", ref)
}

func getTemporarySnapshotterName(ref string) string {
	return fmt.Sprintf("temp-%s", ref)
}

func (s *SnapshotterService) Pull(from, proxy, ref string) (err error) {
	accRef := getAcceleratedSnapshotterName(ref)
	accFrom := ""

	var info snapshots.Info
	info, err = s.sn.Stat(s.ctx, accRef)
	if err == nil {
		log.G(s.ctx).WithField("info", info).Info("found starlight image")
		return nil
	}

	if from != "" {
		info, err = s.sn.Stat(s.ctx, accFrom)
		if err != nil {
			log.G(s.ctx).
				WithError(err).
				WithField("image", from).
				Error("failed to get status of the base image (maybe it does not exists)")
			return errors.Wrap(err, "failed to get status of the base image")
		}
		accFrom = getAcceleratedSnapshotterName(from)
	}

	labels := map[string]string{
		util.ContainerdGCLabel: time.Now().UTC().Format(time.RFC3339),
		util.SnapshotterLabel:  util.Version,
		util.ProxyLabel:        proxy,
	}

	// "" -> ref()
	var mnt []mount.Mount
	if mnt, err = s.sn.Prepare(s.ctx, accFrom, ref, snapshots.WithLabels(labels)); err != nil {
		return err
	}
	// will be blocked until all the necessary files are ready (or reach the readiness flag)
	log.G(s.ctx).WithFields(logrus.Fields{
		"mnt": mnt,
		"ref": ref,
	}).Info("prepared snapshot")

	// ref() -> accelerated-ref()
	if err = s.sn.Commit(s.ctx, ref, accRef, snapshots.WithLabels(labels)); err != nil {
		return err
	}
	log.G(s.ctx).WithFields(logrus.Fields{
		"ref": ref,
	}).Info("committed snapshot")
	// file system will be ready by then

	return nil
}

func (s *SnapshotterService) Create(ref, containerName string, optimize bool) (sn string, mnt []mount.Mount, err error) {
	accRef := getAcceleratedSnapshotterName(ref)

	var info snapshots.Info
	info, err = s.sn.Stat(s.ctx, accRef)
	if err != nil {
		log.G(s.ctx).WithField("info", info).Error("starlight image not found")
		return "", nil, err
	}
	log.G(s.ctx).WithFields(logrus.Fields{
		"ref":    ref,
		"labels": info.Labels,
	}).Info("found starlight snapshot")

	labels := map[string]string{
		util.ContainerdGCLabel: time.Now().UTC().Format(time.RFC3339),
		util.SnapshotterLabel:  info.Labels[util.SnapshotterLabel],
		util.ProxyLabel:        info.Labels[util.ProxyLabel],
	}

	if optimize {
		labels[util.OptimizeLabel] = "True"
	}

	final := s.GetHash(fmt.Sprintf("%s%v", containerName, labels[util.ContainerdGCLabel]))
	if mnt, err = s.sn.Prepare(
		s.ctx,
		accRef,
		final,
		snapshots.WithLabels(labels),
	); err != nil {
		return "", nil, err
	}

	return final, mnt, nil
}

func (s *SnapshotterService) GetHash(ref string) string {
	h := sha256.New()
	h.Write([]byte(ref))
	return fmt.Sprintf("%x.json", h.Sum(nil))
}

func NewSnapshotterService(ctx context.Context, client *containerd.Client) (sn *SnapshotterService) {
	rand.Seed(time.Now().UnixNano())
	sn = &SnapshotterService{
		ctx: ctx,
		sn:  client.SnapshotService("starlight"),
		id:  fmt.Sprintf("%06d", rand.Intn(100000)),
	}

	log.G(ctx).WithFields(logrus.Fields{
		"id": sn.id,
	}).Info("sn service created")
	return
}
