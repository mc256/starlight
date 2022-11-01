/*
   file created by Junlin Chen in 2022

*/

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/containerd/snapshots/storage"
	"github.com/containerd/continuity/fs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Snapshotter struct {
	ctx  context.Context
	cfg  *Configuration
	ms   *storage.MetaStore
	lock sync.Mutex
	ls   *LayerStorage
}

var (
	letters = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
)

func randSequence(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func getRandomId() string {
	return fmt.Sprintf("starlightfs-%s", randSequence(10))
}

type LayerStorage struct {
	Items []layerStorageItem `json:"items"`
}

type layerStorageItem struct {
	Digest       string `json:"digest"`
	Completed    string `json:"completed"`
	completeTime time.Time
}

func (s *Snapshotter) SaveLayerStore() (err error) {
	var buf []byte
	buf, err = json.Marshal(s)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(s.cfg.FileSystemRoot, "layers.json"), buf, 0644)
	if err != nil {
		return err
	}
	return nil
}

func NewSnapshotter(ctx context.Context, cfg *Configuration) (s *Snapshotter, err error) {
	// initialize the snapshotter
	s = &Snapshotter{
		ctx: ctx,
		cfg: cfg,
	}

	// database
	if err := os.MkdirAll(filepath.Dir(cfg.Metadata), 0700); err != nil {
		return nil, err
	}
	s.ms, err = storage.NewMetaStore(cfg.Metadata)
	if err != nil {
		return nil, err
	}

	return
}

func (s *Snapshotter) getSnapshot(name string) (p string, err error) {
	ctx, t, err := s.ms.TransactionContext(s.ctx, false)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create transaction context")
	}
	defer t.Rollback()
	var (
		ss storage.Snapshot
	)
	ss, err = storage.GetSnapshot(ctx, name)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get snapshot")
	}
	fmt.Println(ss)
	return "", nil
}

func (s *Snapshotter) addSnapshot(key, parent string) (err error) {
	ctx, t, err := s.ms.TransactionContext(s.ctx, true)
	if err != nil {
		return errors.Wrapf(err, "failed to create transaction context")
	}
	defer t.Rollback()
	var ss storage.Snapshot
	// parent --> key
	ss, err = storage.CreateSnapshot(ctx, snapshots.KindActive, key, parent)
	log.G(s.ctx).WithFields(logrus.Fields{
		"ID":     ss.ID,
		"Kind":   ss.Kind,
		"Parent": ss.ParentIDs,
	}).Info("create snapshot")

	if err != nil {
		return errors.Wrapf(err, "failed to create snapshot")
	}

	if err = t.Commit(); err != nil {
		return errors.Wrapf(err, "failed to commit transaction")
	}
	return nil
}

func (s *Snapshotter) commitSnapshot(name, key string) (err error) {
	ctx, t, err := s.ms.TransactionContext(s.ctx, true)
	if err != nil {
		return errors.Wrapf(err, "failed to create transaction context")
	}
	defer t.Rollback()

	du, err := fs.DiskUsage(ctx, s.cfg.FileSystemRoot)
	if err != nil {
		return err
	}
	usage := snapshots.Usage(du)

	var ss string
	// key --> name
	ss, err = storage.CommitActive(ctx, key, name, usage)
	log.G(s.ctx).WithFields(logrus.Fields{
		"ID": ss,
	}).Info("commit snapshot")

	if err != nil {
		return errors.Wrapf(err, "failed to commit snapshot")
	}

	if err = t.Commit(); err != nil {
		return errors.Wrapf(err, "failed to commit transaction")
	}
	return nil
}

func (s *Snapshotter) removeSnapshot(key string) (err error) {
	ctx, t, err := s.ms.TransactionContext(s.ctx, true)
	if err != nil {
		return errors.Wrapf(err, "failed to create transaction context")
	}
	defer t.Rollback()
	var (
		ss string
		k  snapshots.Kind
	)
	ss, k, err = storage.Remove(ctx, key)
	log.G(s.ctx).WithFields(logrus.Fields{
		"ID":   ss,
		"Kind": k,
	}).Info("remove snapshot")
	if err != nil {
		return errors.Wrapf(err, "failed to remove snapshot")
	}

	if err = t.Commit(); err != nil {
		return errors.Wrapf(err, "failed to commit transaction")
	}
	return nil
}

// Stat returns the info for an active or committed snapshot by name or
// key.
//
// Should be used for parent resolution, existence checks and to discern
// the kind of snapshot.
func (s *Snapshotter) Stat(ctx context.Context, key string) (snapshots.Info, error) {
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
func (s *Snapshotter) Update(ctx context.Context, info snapshots.Info, fieldpaths ...string) (snapshots.Info, error) {
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
func (s *Snapshotter) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	panic("not implemented")
}

// Mounts returns the mounts for the active snapshot transaction identified
// by key. Can be called on an read-write or readonly transaction. This is
// available only for active snapshots.
//
// This can be used to recover mounts after calling View or Prepare.
func (s *Snapshotter) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {

	panic("not implemented")
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
func (s *Snapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	log.G(s.ctx).WithFields(logrus.Fields{
		"key":    key,
		"parent": parent,
	}).Info("prepare snapshot")

	ctx, t, err := s.ms.TransactionContext(ctx, true)
	if err != nil {
		return nil, err
	}
	ss, err := storage.CreateSnapshot(ctx, snapshots.KindActive, key, parent, opts...)
	if err != nil {
		if rerr := t.Rollback(); rerr != nil {
			log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
		}
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}
	log.G(s.ctx).WithFields(logrus.Fields{
		"ID":     ss.ID,
		"Kind":   ss.Kind,
		"Parent": ss.ParentIDs,
	}).Info("create snapshot - active")
	if err := t.Commit(); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	return []mount.Mount{{}}, nil
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
func (s *Snapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	panic("not implemented")
}

// Commit captures the changes between key and its parent into a snapshot
// identified by name.  The name can then be used with the snapshotter's other
// methods to create subsequent snapshots.
//
// A committed snapshot will be created under name with the parent of the
// active snapshot.
//
// After commit, the snapshot identified by key is removed.
func (s *Snapshotter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error {
	log.G(s.ctx).WithFields(logrus.Fields{
		"name": name,
		"key":  key,
	}).Info("commit snapshot")

	ctx, t, err := s.ms.TransactionContext(ctx, true)
	if err != nil {
		return err
	}

	id, _, _, err := storage.GetInfo(ctx, key)
	if err != nil {
		if rerr := t.Rollback(); rerr != nil {
			log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
		}
		return err
	}
	log.G(s.ctx).WithFields(logrus.Fields{
		"ID": id,
	}).Info("get snapshot info")

	//usage, err := fs.DiskUsage(ctx, s.getSnapshotDir(id))
	usage, err := fs.DiskUsage(ctx, "/tmp/test/0")
	if err != nil {
		if rerr := t.Rollback(); rerr != nil {
			log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
		}
		return err
	}

	if _, err := storage.CommitActive(ctx, key, name, snapshots.Usage(usage), opts...); err != nil {
		if rerr := t.Rollback(); rerr != nil {
			log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
		}
		return fmt.Errorf("failed to commit snapshot: %w", err)
	}
	return t.Commit()
}

// Remove the committed or active snapshot by the provided key.
//
// All resources associated with the key will be removed.
//
// If the snapshot is a parent of another snapshot, its children must be
// removed before proceeding.
func (s *Snapshotter) Remove(ctx context.Context, key string) error {
	panic("not implemented")
}

// Walk will call the provided function for each snapshot in the
// snapshotter which match the provided filters. If no filters are
// given all items will be walked.
// Filters:
//  name
//  parent
//  kind (active,view,committed)
//  labels.(label)
func (s *Snapshotter) Walk(ctx context.Context, fn snapshots.WalkFunc, fs ...string) error {
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
func (s *Snapshotter) Close() error {
	return nil
}
