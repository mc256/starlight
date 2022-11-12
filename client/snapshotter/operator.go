/*
   file created by Junlin Chen in 2022

*/

package snapshotter

import (
	"context"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/snapshots"
	"github.com/mc256/starlight/util"
	"github.com/pkg/errors"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
)

type OperatorClient interface {
	// GetFilesystemRoot gets the directory to store all layers
	GetFilesystemRoot() string

	// AddCompletedLayers find out completed layer and save it to Client's layerMap
	AddCompletedLayers(compressedLayerDigest string)
}

// Operator communicates with the containerd interface.
// Because containerd would cache snapshot information, therefore, we need to communicate with
// containerd to update the information we have. On the other side, the Plugin class communicates
// with the snapshotter plugin interface
type Operator struct {
	ctx    context.Context
	client OperatorClient
	sn     snapshots.Snapshotter
}

func NewOperator(ctx context.Context, client OperatorClient, sn snapshots.Snapshotter) *Operator {
	return &Operator{
		ctx:    ctx,
		client: client,
		sn:     sn,
	}
}

// ScanExistingFilesystems scans place where the extracted file content is stored
// in case the file system has not extracted fully (without the `complete.json` file),
// we will remove the directory.
func (op *Operator) ScanExistingFilesystems() {
	var (
		err                    error
		dir1, dir2, dir3, dir4 []fs.FileInfo
		x1, x2, x3             bool
	)
	log.G(op.ctx).
		WithField("root", op.client.GetFilesystemRoot()).
		Debug("scanning existing filesystems")

	dir1, err = ioutil.ReadDir(filepath.Join(op.client.GetFilesystemRoot(), "layers"))
	if err != nil {
		return
	}
	for _, d1 := range dir1 {
		x1 = false
		if d1.IsDir() && len(d1.Name()) == 1 {
			dir2, err = ioutil.ReadDir(filepath.Join(op.client.GetFilesystemRoot(), "layers",
				d1.Name(),
			))
			if err != nil {
				continue
			}
			for _, d2 := range dir2 {
				x2 = false
				if d2.IsDir() && len(d2.Name()) == 2 {
					dir3, err = ioutil.ReadDir(filepath.Join(op.client.GetFilesystemRoot(), "layers",
						d1.Name(), d2.Name(),
					))
					if err != nil {
						continue
					}
					for _, d3 := range dir3 {
						x3 = false
						if d3.IsDir() && len(d3.Name()) == 2 {
							dir4, err = ioutil.ReadDir(filepath.Join(op.client.GetFilesystemRoot(), "layers",
								d1.Name(), d2.Name(), d3.Name(),
							))
							if err != nil {
								continue
							}
							for _, d4 := range dir4 {
								if d4.IsDir() {
									d := filepath.Join(op.client.GetFilesystemRoot(), "layers",
										d1.Name(), d2.Name(), d3.Name(), d4.Name(),
									)
									h := fmt.Sprintf("sha256:%s%s%s%s",
										d1.Name(), d2.Name(), d3.Name(), d4.Name(),
									)
									completeFile := filepath.Join(d, "completed.json")
									if _, err = os.Stat(completeFile); err != nil {
										_ = os.RemoveAll(filepath.Join(op.client.GetFilesystemRoot(), "layers",
											d1.Name(), d2.Name(), d3.Name(), d4.Name(),
										))
										log.G(op.ctx).WithField("digest", h).Warn("removed incomplete layer")
									} else {
										x1, x2, x3 = true, true, true
										op.client.AddCompletedLayers(h)
										log.G(op.ctx).WithField("digest", h).Debug("found layer")
									}
								}
							}
						}
						if !x3 {
							_ = os.RemoveAll(filepath.Join(op.client.GetFilesystemRoot(), "layers",
								d1.Name(), d2.Name(), d3.Name(),
							))
						}
					}
				}
				if !x2 {
					_ = os.RemoveAll(filepath.Join(op.client.GetFilesystemRoot(), "layers",
						d1.Name(), d2.Name(),
					))
				}
			}
		}
		if !x1 {
			_ = os.RemoveAll(filepath.Join(op.client.GetFilesystemRoot(), "layers",
				d1.Name(),
			))
		}
	}

	return
}

func (op *Operator) ScanSnapshots() (err error) {
	return op.sn.Walk(op.ctx, func(ctx context.Context, info snapshots.Info) (err error) {
		log.G(op.ctx).
			WithField("snapshot", info.Name).
			WithField("parent", info.Parent).
			WithField("labels", info.Labels).
			Debug("found snapshot")
		return
	})
}

func (op *Operator) AddSnapshot(name, parent, imageDigest, uncompressedDigest string, stack int64) (sn *snapshots.Info, err error) {
	var (
		snn snapshots.Info
	)

	randId := util.GetRandomId("prepare")

	// check name exists or not
	snn, err = op.sn.Stat(op.ctx, name)
	if err == nil {
		return &snn, nil
	}

	// parent -> key
	// be aware that the returned mount is Read-Only.
	_, err = op.sn.Prepare(op.ctx, randId, parent, snapshots.WithLabels(map[string]string{
		util.SnapshotLabelRefImage:        imageDigest,
		util.SnapshotLabelRefLayer:        fmt.Sprintf("%d", stack),
		util.SnapshotLabelRefUncompressed: uncompressedDigest,
	}))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create snapshot")
	}

	// key -> name
	err = op.sn.Commit(op.ctx, name, randId, snapshots.WithLabels(map[string]string{
		util.SnapshotLabelRefImage:        imageDigest,
		util.SnapshotLabelRefLayer:        fmt.Sprintf("%d", stack),
		util.SnapshotLabelRefUncompressed: uncompressedDigest,
	}))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to commit snapshot")
	}

	// stat
	snn, err = op.sn.Stat(op.ctx, name)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to stat snapshot")
	}
	return &snn, nil
}
