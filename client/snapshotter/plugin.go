/*
   file created by Junlin Chen in 2022

*/

package snapshotter

import (
	"context"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/containerd/snapshots/storage"
	"github.com/containerd/continuity/fs"
	"github.com/mc256/starlight/util"
	"github.com/mc256/starlight/util/common"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

//////////////////////////////////////////////////////////////////////
// COPY FROM containerd snapshots/overlay/overlay.go
/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

//////////////////////////////////////////////////////////////////////

type PluginClient interface {

	// GetFilesystemPath only use this for starlightfs' stat
	GetFilesystemPath(compressDigest string) string

	// GetMountingPoint returns the path of the snapshotter's mounting point, considering
	// getUpper, getWork, getStarlightFS functions instead of using this function directly
	GetMountingPoint(ssId string) string

	// PrepareManager inform the client to load specified manager in memory.
	// It requires the manifest, Starlight Metadatain and image config is present in containerd's content storage.
	// In case the above requirements are not met, the client should return an error.
	// The plugin should then try the next referenced manager (manifest digest).
	PrepareManager(manifest digest.Digest) (err error)

	// Unmount starlightfs
	Unmount(compressDigest, key string) error

	// Mount starlightfs
	Mount(layerDigest digest.Digest, snapshotId string, sn *snapshots.Info) (mnt string, err error)
}

type Plugin struct {
	ctx    context.Context
	ms     *storage.MetaStore
	lock   sync.Mutex
	client PluginClient
}

func NewPlugin(ctx context.Context, client PluginClient, metadataDB string) (s *Plugin, err error) {
	// initialize the snapshotter
	s = &Plugin{
		ctx:    ctx,
		client: client,
	}

	// database
	if err := os.MkdirAll(filepath.Dir(metadataDB), 0700); err != nil {
		return nil, err
	}
	s.ms, err = storage.NewMetaStore(metadataDB)
	if err != nil {
		return nil, err
	}

	// overlayfs

	return
}

func (s *Plugin) getUpper(ssId string) string {
	return filepath.Join(s.client.GetMountingPoint(ssId), "upper")
}
func (s *Plugin) getWork(ssId string) string {
	return filepath.Join(s.client.GetMountingPoint(ssId), "work")
}
func (s *Plugin) getStarlightFS(ssId string) string {
	return filepath.Join(s.client.GetMountingPoint(ssId), "slfs")
}

func (s *Plugin) getMountingPoint(ssId string) string {
	return s.client.GetMountingPoint(ssId)
}

//////////////////////////////////////////////////////////////////////

func (s *Plugin) Stat(ctx context.Context, key string) (snapshots.Info, error) {
	c, t, err := s.ms.TransactionContext(ctx, false)
	if err != nil {
		return snapshots.Info{}, err
	}
	defer t.Rollback()
	_, info, _, err := storage.GetInfo(c, key)
	if err != nil {
		return snapshots.Info{}, err
	}

	log.G(s.ctx).WithFields(logrus.Fields{
		"name": info.Name,
	}).Debug("sn: stat")
	return info, nil
}

func (s *Plugin) Update(ctx context.Context, info snapshots.Info, fieldpaths ...string) (snapshots.Info, error) {
	c, t, err := s.ms.TransactionContext(ctx, true)
	if err != nil {
		return snapshots.Info{}, err
	}

	info, err = storage.UpdateInfo(c, info, fieldpaths...)
	if err != nil {
		t.Rollback()
		return snapshots.Info{}, err
	}

	if err = t.Commit(); err != nil {
		return snapshots.Info{}, err
	}

	log.G(s.ctx).WithFields(logrus.Fields{
		"name":  info.Name,
		"usage": fieldpaths,
	}).Debug("sn: updated")
	return info, nil
}

func (s *Plugin) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	c, t, err := s.ms.TransactionContext(ctx, false)
	if err != nil {
		return snapshots.Usage{}, err
	}
	defer t.Rollback()
	snId, inf, usage, err := storage.GetInfo(c, key)
	if err != nil {
		return snapshots.Usage{}, err
	}

	if _, usingSL := inf.Labels[util.SnapshotLabelRefUncompressed]; !usingSL {
		// overlayfs
		if inf.Kind == snapshots.KindActive {
			var du fs.Usage
			du, err = fs.DiskUsage(c, s.getUpper(snId))
			usage = snapshots.Usage(du)
		}
	} else {
		// starlightfs is RO so we don't need to calculate the usage
	}

	log.G(s.ctx).WithFields(logrus.Fields{
		"key":   key,
		"usage": usage,
	}).Debug("sn: usage")
	return usage, nil
}

func (s *Plugin) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {
	c, t, err := s.ms.TransactionContext(ctx, false)
	if err != nil {
		return nil, err
	}
	defer t.Rollback()
	var (
		ssId string
		info snapshots.Info
	)
	ssId, info, _, err = storage.GetInfo(c, key)
	if err != nil {
		return nil, err
	}

	mnt, err := s.mounts(c, ssId, &info)
	if err != nil {
		return nil, err
	}
	log.G(ctx).WithFields(logrus.Fields{
		"key": key,
		"mnt": mnt,
	}).Debug("sn: mount")
	return mnt, nil
}

func (s *Plugin) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return s.newSnapshot(ctx, key, parent, false, opts...)
}

func (s *Plugin) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return s.newSnapshot(ctx, key, parent, true, opts...)
}

func (s *Plugin) newSnapshot(ctx context.Context, key, parent string, readonly bool, opts ...snapshots.Opt) ([]mount.Mount, error) {

	log.G(s.ctx).WithFields(logrus.Fields{
		"key":       key,
		"parent":    parent,
		"_readonly": readonly,
	}).Debug("sn: prepare")

	c, t, err := s.ms.TransactionContext(ctx, true)
	if err != nil {
		return nil, err
	}

	kind := snapshots.KindActive
	if readonly {
		kind = snapshots.KindView
	}

	var (
		info    snapshots.Info
		usingSL bool
	)
	for _, o := range opts {
		if err = o(&info); err != nil {
			return nil, err
		}
	}
	// create snapshot
	ss, err := storage.CreateSnapshot(c, kind, key, parent, opts...)
	if err != nil {
		if rerr := t.Rollback(); rerr != nil {
			log.G(ctx).WithError(rerr).Warn("sn: failed to rollback transaction")
		}
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	// create snapshot directories
	if err = os.MkdirAll(s.getStarlightFS(ss.ID), 0755); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(s.getUpper(ss.ID), 0755); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(s.getWork(ss.ID), 0711); err != nil {
		return nil, err
	}

	var inf snapshots.Info
	_, inf, _, err = storage.GetInfo(c, key)
	if err != nil {
		return nil, err
	}

	// mount snapshot
	mnt, err := s.mounts(c, ss.ID, &inf)
	if err != nil {
		log.G(s.ctx).WithError(err).Error("sn: mount failed")
		return nil, err
	}

	// commit changes
	if err := t.Commit(); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	log.G(s.ctx).WithFields(logrus.Fields{
		"key":        key,
		"parent":     parent,
		"_readonly":  readonly,
		"id":         ss.ID,
		"_starlight": usingSL,
	}).Debug("sn: prepared")

	return mnt, nil
}

func (s *Plugin) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) (err error) {
	var (
		c          context.Context
		t          storage.Transactor
		inf        snapshots.Info
		usage      fs.Usage
		snId, unId string

		usingSL bool
	)

	c, t, err = s.ms.TransactionContext(ctx, true)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if rerr := t.Rollback(); rerr != nil {
				log.G(ctx).WithError(rerr).Warn("sn: failed to rollback transaction")
			}
		}
	}()

	// Get Snapshot info
	snId, inf, _, err = storage.GetInfo(c, key)
	if err != nil {
		return err
	}

	// update info so we can get Starlight related labels
	for _, opt := range opts {
		if err = opt(&inf); err != nil {
			return err
		}
	}

	// Disk Usage
	if unId, usingSL = inf.Labels[util.SnapshotLabelRefUncompressed]; usingSL {
		// starlight - the entire decompressed layer count but not accurate because
		// it might be in the process of being decompressed using the manager.Extract() function
		usage, err = fs.DiskUsage(c, s.client.GetFilesystemPath(unId))
		if err != nil {
			if rerr := t.Rollback(); rerr != nil {
				log.G(ctx).WithError(rerr).Warn("sn: failed to rollback transaction")
			}
			return err
		}
	} else {
		// overlay - only upper layer counts
		usage, err = fs.DiskUsage(c, s.getUpper(snId))
		if err != nil {
			return err
		}
	}

	// Commit
	if _, err = storage.CommitActive(c, key, name, snapshots.Usage(usage), opts...); err != nil {
		return fmt.Errorf("failed to commit snapshot: %w", err)
	}

	log.G(s.ctx).WithFields(logrus.Fields{
		"name":  name,
		"key":   key,
		"usage": usage,
	}).Debug("sn: committed")
	return t.Commit()
}

// Remove the committed or active snapshot by the provided key.
//
// All resources associated with the key will be removed.
//
// If the snapshot is a parent of another snapshot, its children must be
// removed before proceeding.
func (s *Plugin) Remove(ctx context.Context, key string) (err error) {
	var (
		c          context.Context
		t          storage.Transactor
		snId, unId string
		usingSL    bool
		info       snapshots.Info
	)
	c, t, err = s.ms.TransactionContext(ctx, true)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if rerr := t.Rollback(); rerr != nil {
				log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
			}
		}
	}()

	// Get Snapshot info
	snId, info, _, err = storage.GetInfo(c, key)
	if err != nil {
		return err
	}

	// remove from database
	_, _, err = storage.Remove(c, key)
	if err = t.Commit(); err != nil {
		return err
	}

	// Remove Filesystems
	if unId, usingSL = info.Labels[util.SnapshotLabelRefUncompressed]; usingSL {
		// starlight
		err = s.client.Unmount(unId, key)
		if err != nil {
			log.G(s.ctx).WithError(err).WithFields(logrus.Fields{
				"key": key,
			}).Warn("sn: failed to remove snapshot mounting")
			return err
		}
	} else {
		// overlayfs
		if err = os.RemoveAll(s.getMountingPoint(snId)); err != nil {
			log.G(s.ctx).WithError(err).WithFields(logrus.Fields{
				"key": key,
			}).Warn("sn: failed to remove snapshot mounting")
			return err
		}
	}

	// log
	log.G(s.ctx).WithFields(logrus.Fields{
		"id":         info.Name,
		"parents":    info.Parent,
		"_starlight": usingSL,
	}).Debug("sn: remove")

	return nil
}

// Walk will call the provided function for each snapshot in the
// snapshotter which match the provided filters. If no filters are
// given all items will be walked.
// Filters:
//  name
//  parent
//  kind (active,view,committed)
//  labels.(label)
func (s *Plugin) Walk(ctx context.Context, fn snapshots.WalkFunc, fs ...string) error {
	c, t, err := s.ms.TransactionContext(ctx, false)
	if err != nil {
		return err
	}
	defer t.Rollback()
	return storage.WalkInfo(c, fn, fs...)
}

// Close releases the internal resources.
//
// Close is expected to be called on the end of the lifecycle of the snapshotter,
// but not mandatory.
//
// Close returns nil when it is already closed.
func (s *Plugin) Close() error {
	return nil
}

func (s *Plugin) getStarlightFeature(info *snapshots.Info) (
	imageDigest, unDigest string,
	stack int64,
	err error) {

	stack, err = strconv.ParseInt(info.Labels[util.SnapshotLabelRefLayer], 10, 64)
	if err != nil {
		return "", "", 0, err
	}

	if info.Labels[util.SnapshotLabelRefImage] == "" || info.Labels[util.SnapshotLabelRefUncompressed] == "" {
		return "", "", 0, nil
	}

	return info.Labels[util.SnapshotLabelRefImage],
		info.Labels[util.SnapshotLabelRefUncompressed],
		stack, nil
}

type SnapshotItem struct {
	ssId string
	inf  snapshots.Info
}

func (si SnapshotItem) IsStarlightFS() bool {
	und, ok := si.inf.Labels[util.SnapshotLabelRefUncompressed]
	return ok && und != ""
}

// GetStarlightFeature returns manifest digest, uncompressed digest, and stack number
func (si SnapshotItem) GetStarlightFeature() (md, und digest.Digest, stack int64) {
	return digest.Digest(si.inf.Labels[util.SnapshotLabelRefImage]),
		digest.Digest(si.inf.Labels[util.SnapshotLabelRefUncompressed]), stack
}

func (s *Plugin) getFsStack(ctx context.Context, cur *snapshots.Info) (pSi []*SnapshotItem, err error) {
	pSi = make([]*SnapshotItem, 0)
	for {
		if cur.Parent == "" {
			break
		}
		item := &SnapshotItem{}
		item.ssId, item.inf, _, err = storage.GetInfo(ctx, cur.Parent)
		if err != nil {
			return nil, err
		}
		cur = &item.inf
		pSi = append(pSi, item)
	}
	return pSi, nil
}

func (s *Plugin) mounts(ctx context.Context, ssId string, inf *snapshots.Info) (mnt []mount.Mount, err error) {
	stack, err := s.getFsStack(ctx, inf)
	// from upper to lower, not include current layer

	if err != nil {
		return nil, err
	}

	current := SnapshotItem{
		ssId: ssId,
		inf:  *inf,
	}

	if len(stack) == 0 {
		var m string
		if current.IsStarlightFS() {
			// starlightfs
			md, und, _ := current.GetStarlightFeature()
			if err = s.client.PrepareManager(md); err != nil {
				return nil, err
			}
			m, err = s.client.Mount(und, ssId, inf)
			if err != nil {
				return nil, err
			}
			return []mount.Mount{
				{
					Type:    "bind",
					Source:  m,
					Options: []string{"rbind", "ro"},
				},
			}, nil
		} else {
			// overlayfs
			rwo := "ro"
			if inf.Kind == snapshots.KindActive {
				rwo = "rw"
			}
			log.G(s.ctx).WithFields(logrus.Fields{
				"key": ssId,
			}).Warn("sn: starlightfs is using as overlayfs")
			return []mount.Mount{
				{
					Type:    "bind",
					Source:  s.getUpper(ssId),
					Options: []string{"rbind", rwo},
				},
			}, nil
		}
	}

	// Looking for manifest digest
	// manifest digest should be determined by the top layer of the lower dirs.
	// it is rare that an image's upper layer are reusing other images layer but we cannot avoid this case.
	// so:
	// 1. all the referenced manifest will be checked and loaded.
	// 2. if the referenced manifest does not exists in the content store, labels will be updated
	mdsm := map[string]bool{}
	mdsl := make([]digest.Digest, 0)
	for _, si := range stack {
		md, _, _ := si.GetStarlightFeature()
		if _, has := mdsm[md.String()]; !has {
			mdsm[md.String()] = true
			mdsl = append(mdsl, md)
		}
	}
	mdsidx := 0

	log.G(s.ctx).
		WithField("managers", mdsl).
		Debug("sn: prepare manager")

	lower := make([]string, 0)
	for i := len(stack) - 1; i >= 0; i-- {
		var m string
		if stack[i].IsStarlightFS() {
			// starlight layer
			_, und, _ := stack[i].GetStarlightFeature()
			for {
				m, err = s.client.Mount(und, stack[i].ssId, &stack[i].inf)
				if err == nil {
					break
				} else if err == common.ErrNoManager {
					// if there is no manager for the layer, prepare a new one
					// prepare the next available manager
					for {
						if mdsidx >= len(mdsl) {
							return nil, errors.Wrapf(err, "no manager for %s", und)
						}
						err = s.client.PrepareManager(mdsl[mdsidx])
						if err == nil {
							break
						}
						mdsidx += 1
					}
				} else {
					return nil, err
				}
			}

			if err != nil {
				return nil, err
			}
		} else {
			// standard overlay fs layer
			m = s.getUpper(stack[i].ssId)
		}
		lower = append(lower, m)
	}

	var options []string
	if inf.Kind == snapshots.KindActive {
		options = append(options,
			fmt.Sprintf("workdir=%s", s.getWork(ssId)),
			fmt.Sprintf("upperdir=%s", s.getUpper(ssId)),
		)
	}

	options = append(options, fmt.Sprintf("lowerdir=%s", strings.Join(lower, ":")))
	return []mount.Mount{
		{
			Type:    "overlay",
			Source:  "overlay",
			Options: options,
		},
	}, nil
}
