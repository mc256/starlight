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
