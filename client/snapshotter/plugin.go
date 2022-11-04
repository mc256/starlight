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
	"sync"
)

type PluginClient interface {
	GetFilesystemPath(compressDigest string) string
	RemoveMounting(compressDigest, key string) error
	GetMountingPoint(ssId string) string
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

//////////////////////////////////////////////////////////////////////

// Stat returns the info for an active or committed snapshot by name or
// key.
//
// Should be used for parent resolution, existence checks and to discern
// the kind of snapshot.
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
	return info, nil
}

// Update updates the info for a snapshot.
//
// Only mutable properties of a snapshot may be updated.
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
	return info, nil
}

// Usage returns the resource usage of an active or committed snapshot
// excluding the usage of parent snapshots.
//
// The running time of this call for active snapshots is dependent on
// implementation, but may be proportional to the size of the resource.
// Callers should take this into consideration. Implementations should
// attempt to honer context cancellation and avoid taking locks when making
// the calculation.
func (s *Plugin) Usage(ctx context.Context, key string) (usage snapshots.Usage, err error) {
	c, t, err := s.ms.TransactionContext(ctx, false)
	if err != nil {
		return snapshots.Usage{}, err
	}
	defer t.Rollback()
	_, info, _, err := storage.GetInfo(c, key)
	if err != nil {
		return snapshots.Usage{}, err
	}

	unId := info.Labels[util.SnapshotLabelRefUncompressed]
	if unId == "" {
		return snapshots.Usage{}, fmt.Errorf("uncompressed snapshot id not found")
	}

	var u fs.Usage
	u, err = fs.DiskUsage(c, s.client.GetFilesystemPath(unId))
	if err != nil {
		if rerr := t.Rollback(); rerr != nil {
			log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
		}
		return snapshots.Usage{}, err
	}

	return snapshots.Usage(u), nil
}

// Mounts returns the mounts for the active snapshot transaction identified
// by key. Can be called on an read-write or readonly transaction. This is
// available only for active snapshots.
//
// This can be used to recover mounts after calling View or Prepare.
func (s *Plugin) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {
	return nil, nil
}

// Prepare creates an active snapshot identified by key descending from the
// provided parent.  The returned mounts can be used to mount the snapshot
// to capture changes.
//
// If a parent is provided, after performing the mounts, the destination
// will start with the content of the parent. The parent must be a
// committed snapshot. Changes to the mounted destination will be captured
// in relation to the parent. The default parent, "", is an empty
// directory.
//
// The changes may be saved to a committed snapshot by calling Commit. When
// one is done with the transaction, Remove should be called on the key.
//
// Multiple calls to Prepare or View with the same key should fail.
//
// using diff_ids in config.json
func (s *Plugin) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return s.newSnapshot(ctx, key, parent, false, opts...)
}

// View behaves identically to Prepare except the result may not be
// committed back to the snapshot snapshotter. View returns a readonly view on
// the parent, with the active snapshot being tracked by the given key.
//
// This method operates identically to Prepare, except that Mounts returned
// may have the readonly flag set. Any modifications to the underlying
// filesystem will be ignored. Implementations may perform this in a more
// efficient manner that differs from what would be attempted with
// `Prepare`.
//
// Commit may not be called on the provided key and will return an error.
// To collect the resources associated with key, Remove must be called with
// key as the argument.
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
	ss, err := storage.CreateSnapshot(c, kind, key, parent, opts...)
	if err != nil {
		if rerr := t.Rollback(); rerr != nil {
			log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
		}
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}
	if err := t.Commit(); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	log.G(s.ctx).WithFields(logrus.Fields{
		"key":      key,
		"parent":   parent,
		"readonly": readonly,
		"id":       ss.ID,
	}).Info("prepared snapshot")

	return []mount.Mount{}, nil
}

// Commit captures the changes between key and its parent into a snapshot
// identified by name.  The name can then be used with the snapshotter's other
// methods to create subsequent snapshots.
//
// A committed snapshot will be created under name with the parent of the
// active snapshot.
//
// After commit, the snapshot identified by key is removed.
func (s *Plugin) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) (err error) {
	var (
		c     context.Context
		t     storage.Transactor
		info  snapshots.Info
		usage fs.Usage
	)

	c, t, err = s.ms.TransactionContext(ctx, true)
	if err != nil {
		return err
	}

	_, _, _, err = storage.GetInfo(c, key)
	if err != nil {
		if err = t.Rollback(); err != nil {
			log.G(ctx).WithError(err).Warn("failed to rollback transaction")
		}
		return err
	}

	for _, opt := range opts {
		if err = opt(&info); err != nil {
			return err
		}
	}

	unId := info.Labels[util.SnapshotLabelRefUncompressed]
	if unId == "" {
		return fmt.Errorf("uncompressed snapshot id not found")
	}

	usage, err = fs.DiskUsage(c, s.client.GetFilesystemPath(unId))
	if err != nil {
		if rerr := t.Rollback(); rerr != nil {
			log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
		}
		return err
	}

	if _, err = storage.CommitActive(c, key, name, snapshots.Usage(usage), opts...); err != nil {
		if rerr := t.Rollback(); rerr != nil {
			log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
		}
		return fmt.Errorf("failed to commit snapshot: %w", err)
	}

	log.G(s.ctx).WithFields(logrus.Fields{
		"name": name,
		"key":  key,
	}).Info("committed snapshot")

	return t.Commit()
}

// Remove the committed or active snapshot by the provided key.
//
// All resources associated with the key will be removed.
//
// If the snapshot is a parent of another snapshot, its children must be
// removed before proceeding.
func (s *Plugin) Remove(ctx context.Context, key string) error {
	c, t, err := s.ms.TransactionContext(ctx, true)
	if err != nil {
		return err
	}
	defer t.Rollback()

	// clear directory
	_, info, _, err := storage.GetInfo(c, key)
	if err != nil {
		return err
	}

	unId := info.Labels[util.SnapshotLabelRefUncompressed]
	if unId == "" {
		return fmt.Errorf("uncompressed snapshot id not found")
	}

	err = s.client.RemoveMounting(unId, key)
	if err != nil {
		log.G(s.ctx).WithError(err).WithFields(logrus.Fields{
			"key": key,
		}).Warn("failed to remove snapshot mounting")
	}

	// remove from dtabase
	_, _, err = storage.Remove(c, key)
	if err != nil {
		return fmt.Errorf("failed to remove: %w", err)
	}
	return t.Commit()
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
