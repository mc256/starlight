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
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
	"math/rand"
	"sort"
	"strings"
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
	ctx     context.Context
	sn      snapshots.Snapshotter
	unique  string
	noGcOpt snOptFunc
}

// PrepareDeltaImage gets
func (s *SnapshotterService) PrepareDeltaImage(fromImages, toImages string) error {
	// sort image orders
	arrFrom := strings.Split(fromImages, ",")
	sort.Strings(arrFrom)
	fromImages = strings.Join(arrFrom, ",")

	arrTo := strings.Split(toImages, ",")
	sort.Strings(arrTo)
	toImages = strings.Join(arrTo, ",")

	var target, source string
	if fromImages == "" {
		source = ""
	} else {
		source = fmt.Sprintf("accelerated(%s)-XXXXXX", fromImages) // committed - name
	}
	target = fmt.Sprintf("target(%s)-%s", toImages, s.unique) // active - key

	// check whether accelerated image exists
	accelerated := fmt.Sprintf("accelerated(%s)-XXXXXX", toImages)
	info, err := s.sn.Stat(s.ctx, accelerated)
	if err == nil {
		log.G(s.ctx).WithField("info", info).Info("found accelerated snapshot")
		return nil
	}

	// accelerated()-X -> target()-rand()
	if _, err := s.sn.Prepare(s.ctx, target, source, s.noGcOpt()); err != nil {
		return err
	}

	// target()-rand() -> accelerated()-X
	if err := s.sn.Commit(s.ctx, accelerated, target, s.noGcOpt()); err != nil {
		return err
	}

	return nil
}

func (s *SnapshotterService) PrepareContainerSnapshot(name, tag, acceleratedImages string, temperature int, optimize bool, optimizeGroup string) (sn string, mnt []mount.Mount, err error) {
	arrAcc := strings.Split(acceleratedImages, ",")
	sort.Strings(arrAcc)
	acceleratedImages = strings.Join(arrAcc, ",")
	committed := fmt.Sprintf("worker-%s-%s-%06d-%s", name, tag, rand.Intn(1000000), s.unique)
	accelerated := fmt.Sprintf("accelerated(%s)-XXXXXX", acceleratedImages)

	labels := map[string]string{
		util.ImageNameLabel:  name,
		util.ImageTagLabel:   tag,
		util.CheckpointLabel: fmt.Sprintf("v%d", temperature),
	}
	if optimize {
		labels[util.OptimizeLabel] = "True"
		labels[util.OptimizeGroupLabel] = optimizeGroup
	}

	if mnt, err = s.sn.Prepare(
		s.ctx,
		committed,
		accelerated,
		snapshots.WithLabels(labels),
		s.noGcOpt(),
	); err != nil {
		return "", nil, err
	}

	return committed, mnt, nil
}

func (s *SnapshotterService) CommitWorker(sn string) error {
	return s.sn.Commit(s.ctx, fmt.Sprintf("%s-XXXXXX", sn[:len(sn)-7]), sn, s.noGcOpt())
}

func NewSnapshotterService(ctx context.Context, client *containerd.Client) (sn *SnapshotterService) {
	rand.Seed(time.Now().Unix())
	sn = &SnapshotterService{
		ctx:    ctx,
		sn:     client.SnapshotService("starlight"),
		unique: fmt.Sprintf("%06d", rand.Intn(100000)),
		noGcOpt: func() snapshots.Opt {
			return snapshots.WithLabels(map[string]string{
				"containerd.io/gc.root": time.Now().UTC().Format(time.RFC3339),
			})
		},
	}

	log.G(ctx).WithFields(logrus.Fields{
		"unique": sn.unique,
	}).Info("sn service created")
	return
}
