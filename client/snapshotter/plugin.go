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
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type PluginClient interface {

	// GetFilesystemPath only use this for starlightfs' stat
	GetFilesystemPath(compressDigest string) string

	// GetMountingPoint returns the path of the snapshotter's mounting point, considering
	// getUpper, getWork, getStarlightFS functions instead of using this function directly
	GetMountingPoint(ssId string) string

	// Unmount starlightfs
	Unmount(compressDigest, key string) error

	// Mount starlightfs
	Mount(md, ld, ssId string, sn *snapshots.Info) (mnt string, err error)
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
	}).Info("stat")
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
	}).Info("updated")
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
		if inf.Kind == snapshots.KindActive {
			var du fs.Usage
			du, err = fs.DiskUsage(c, s.getUpper(snId))
			usage = snapshots.Usage(du)
		}
	}

	log.G(s.ctx).WithFields(logrus.Fields{
		"key":   key,
		"usage": usage,
	}).Info("usage")
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
	}).Info("mount")
	return mnt, nil
}

func (s *Plugin) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return s.newSnapshot(ctx, key, parent, false, opts...)
}

func (s *Plugin) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return s.newSnapshot(ctx, key, parent, true, opts...)
}

func (s *Plugin) newSnapshot(ctx context.Context, key, parent string, readonly bool, opts ...snapshots.Opt) ([]mount.Mount, error) {

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
			log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
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
		return nil, err
	}

	// commit changes
	if err := t.Commit(); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	log.G(s.ctx).WithFields(logrus.Fields{
		"key":       key,
		"parent":    parent,
		"readonly":  readonly,
		"id":        ss.ID,
		"starlight": usingSL,
		"mnt":       mnt,
	}).Info("prepared")

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

	// Get Snapshot info
	snId, inf, _, err = storage.GetInfo(c, key)
	if err != nil {
		if err = t.Rollback(); err != nil {
			log.G(ctx).WithError(err).Warn("failed to rollback transaction")
		}
		return err
	}

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
				log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
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
		if rerr := t.Rollback(); rerr != nil {
			log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
		}
		return fmt.Errorf("failed to commit snapshot: %w", err)
	}

	log.G(s.ctx).WithFields(logrus.Fields{
		"name":  name,
		"key":   key,
		"usage": usage,
	}).Info("committed")
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
	defer t.Rollback()

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
			}).Warn("failed to remove snapshot mounting")
			return err
		}
	} else {
		// overlay
		if err = os.RemoveAll(s.getMountingPoint(snId)); err != nil {
			log.G(s.ctx).WithError(err).WithFields(logrus.Fields{
				"key": key,
			}).Warn("failed to remove snapshot mounting")
			return err
		}
	}

	// log
	log.G(s.ctx).WithFields(logrus.Fields{
		"key":       key,
		"id":        info.Name,
		"parents":   info.Parent,
		"starlight": usingSL,
	}).Info("remove")

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

func (si SnapshotItem) GetStarlightFeature() (md, und string, stack int64) {
	return si.inf.Labels[util.SnapshotLabelRefImage], si.inf.Labels[util.SnapshotLabelRefUncompressed], stack
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
	stack, err := s.getFsStack(ctx, inf) // from upper to lower
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
			md, und, _ := current.GetStarlightFeature()
			m, err = s.client.Mount(md, und, ssId, inf)
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
			return nil, fmt.Errorf("please use native overlayfs")
		}
	}

	lower := make([]string, 0)
	for i := len(stack) - 1; i >= 0; i-- {
		var m string
		if stack[i].IsStarlightFS() {
			// starlight layer
			md, und, _ := stack[i].GetStarlightFeature()
			m, err = s.client.Mount(md, und, stack[i].ssId, &stack[i].inf)
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
	} else {
		var m string
		if current.IsStarlightFS() {
			md, und, _ := current.GetStarlightFeature()
			m, err = s.client.Mount(und, md, ssId, inf)
			if err != nil {
				return nil, err
			}
		} else {
			m = s.getUpper(ssId)
		}
		lower = append(lower, m)
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
