/*
   file created by Junlin Chen in 2022

*/

package client

import (
	"context"
	"fmt"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
)

type Snapshotter struct {
}

func NewSnapshotter() *Snapshotter {
	return &Snapshotter{}
}

func (s *Snapshotter) findOutWhy() {
	fmt.Println("find out why")
}

// Stat returns the info for an active or committed snapshot by name or
// key.
//
// Should be used for parent resolution, existence checks and to discern
// the kind of snapshot.
func (s *Snapshotter) Stat(ctx context.Context, key string) (snapshots.Info, error) {
	s.findOutWhy()
	panic("not implemented")
}

// Update updates the info for a snapshot.
//
// Only mutable properties of a snapshot may be updated.
func (s *Snapshotter) Update(ctx context.Context, info snapshots.Info, fieldpaths ...string) (snapshots.Info, error) {
	s.findOutWhy()
	panic("not implemented")
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
	s.findOutWhy()
	panic("not implemented")
}

// Mounts returns the mounts for the active snapshot transaction identified
// by key. Can be called on an read-write or readonly transaction. This is
// available only for active snapshots.
//
// This can be used to recover mounts after calling View or Prepare.
func (s *Snapshotter) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {
	s.findOutWhy()
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
func (s *Snapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	s.findOutWhy()
	panic("not implemented")
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
	s.findOutWhy()
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
	s.findOutWhy()
	panic("not implemented")
}

// Remove the committed or active snapshot by the provided key.
//
// All resources associated with the key will be removed.
//
// If the snapshot is a parent of another snapshot, its children must be
// removed before proceeding.
func (s *Snapshotter) Remove(ctx context.Context, key string) error {
	s.findOutWhy()
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
func (s *Snapshotter) Walk(ctx context.Context, fn snapshots.WalkFunc, filters ...string) error {
	s.findOutWhy()
	panic("not implemented")
}

// Close releases the internal resources.
//
// Close is expected to be called on the end of the lifecycle of the snapshotter,
// but not mandatory.
//
// Close returns nil when it is already closed.
func (s *Snapshotter) Close() error {
	s.findOutWhy()
	panic("not implemented")
}
