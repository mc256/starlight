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

package client

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/containerd/snapshots/storage"
	"github.com/containerd/continuity/fs"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	starlightfs "github.com/mc256/starlight/fs"
	"github.com/mc256/starlight/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.etcd.io/bbolt"
)

type snapshotter struct {
	gCtx context.Context

	ms *storage.MetaStore
	db *bbolt.DB

	layerStore *starlightfs.LayerStore
	receiver   map[string]*starlightfs.Receiver

	//imageReadersMux sync.Mutex
	fsMap   map[string]*starlightfs.FsInstance
	fsTrace bool

	cfg              *Configuration
	proxyConnections map[string]*ProxyConfig
}

// NewSnapshotter returns a Snapshotter which copies layers on the underlying
// file system. A metadata file is stored under the root.
func NewSnapshotter(ctx context.Context, cfg *Configuration) (snapshots.Snapshotter, error) {
	if err := os.MkdirAll(cfg.FileSystemRoot, 0700); err != nil {
		return nil, err
	}

	// containerd snapshot database
	ms, err := storage.NewMetaStore(cfg.Metadata + ".sn")
	if err != nil {
		return nil, err
	}

	// starlight metadata database
	db, err := util.OpenDatabase(ctx, cfg.Metadata+".sl")
	if err != nil {
		return nil, err
	}

	// root path for starlight fs
	layerStore, err := starlightfs.NewLayerStore(ctx, db, filepath.Join(cfg.FileSystemRoot, "sfs"))
	if err != nil {
		return nil, err
	}

	if err := os.Mkdir(filepath.Join(cfg.FileSystemRoot, "sfs"), 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	return &snapshotter{
		gCtx: ctx,

		ms: ms,
		db: db,

		layerStore: layerStore,

		receiver: make(map[string]*starlightfs.Receiver, 0),
		fsMap:    make(map[string]*starlightfs.FsInstance, 0),
	}, nil
}

func (o *snapshotter) Stat(ctx context.Context, key string) (snapshots.Info, error) {
	log.G(ctx).WithField("key", key).Info("stat")
	ctx, t, err := o.ms.TransactionContext(ctx, false)
	if err != nil {
		return snapshots.Info{}, err
	}
	defer t.Rollback()
	_, info, _, err := storage.GetInfo(ctx, key)
	if err != nil {
		return snapshots.Info{}, err
	}
	return info, nil
}

func (o *snapshotter) Update(ctx context.Context, info snapshots.Info, fieldpaths ...string) (snapshots.Info, error) {
	log.G(ctx).WithField("info", info).WithField("fieldPaths", fieldpaths).Info("update")
	ctx, t, err := o.ms.TransactionContext(ctx, true)
	if err != nil {
		return snapshots.Info{}, err
	}
	defer t.Rollback()
	info, err = storage.UpdateInfo(ctx, info, fieldpaths...)
	if err != nil {
		return snapshots.Info{}, err
	}

	if err := t.Commit(); err != nil {
		return snapshots.Info{}, err
	}
	return info, nil
}

// Walk the committed snapshots.
func (o *snapshotter) Walk(ctx context.Context, fn snapshots.WalkFunc, fs ...string) error {
	log.G(ctx).WithField("fs", fs).Info("walk")
	ctx, t, err := o.ms.TransactionContext(ctx, false)
	if err != nil {
		return err
	}
	defer t.Rollback()
	return storage.WalkInfo(ctx, fn, fs...)
}

// Close closes the snapshotter
func (o *snapshotter) Close() error {
	for k, v := range o.fsMap {
		if err := v.GetServer().Unmount(); err != nil {
			log.G(o.gCtx).Error(err)
		} else {
			log.G(o.gCtx).WithField("fs", v.GetMountPoint()).WithField("k", k).Info("umounted")
		}
	}

	_ = o.ms.Close()
	err2 := o.db.Close()
	return err2
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
func (o *snapshotter) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	log.G(ctx).WithField("key", key).Info("usage")
	ctx, t, err := o.ms.TransactionContext(ctx, false)
	if err != nil {
		return snapshots.Usage{}, err
	}
	defer t.Rollback()

	id, info, usage, err := storage.GetInfo(ctx, key)
	if err != nil {
		return snapshots.Usage{}, err
	}

	if info.Kind == snapshots.KindActive {
		dir := o.getSnDir(id)
		du, err := fs.DiskUsage(ctx, dir) // This only reports the rw layer of the image
		if err != nil {
			return snapshots.Usage{}, err
		}

		usage = snapshots.Usage(du)
	}
	return usage, nil
}

func (o *snapshotter) getSnDir(id string) string {
	return path.Join(o.cfg.FileSystemRoot, "sfs", id)
}

// pullImage is the fist step get the Starlight Image
// this method is referenced by Prepare()
func (o *snapshotter) pullImage(ctx context.Context, key, parent, _key, _parent string, mnt []mount.Mount, opts ...snapshots.Opt) ([]mount.Mount, error) {
	var targetImage, sourceImage string
	parentBase := path.Base(_parent)
	keyBase := path.Base(_key)
	if _parent == "" {
		sourceImage = "_"
	} else if strings.HasPrefix(parentBase, "accelerated(") && strings.HasSuffix(parentBase, ")") {
		sourceImage = strings.TrimSuffix(strings.TrimPrefix(parentBase, "accelerated("), ")")
	} else {
		return nil, util.ErrUnknownSnapshotParameter
	}

	if strings.HasPrefix(keyBase, "target(") && strings.HasSuffix(keyBase, ")") {
		targetImage = strings.TrimSuffix(strings.TrimPrefix(keyBase, "target("), ")")
	} else {
		return nil, util.ErrUnknownSnapshotParameter
	}

	sn, err := storage.CreateSnapshot(ctx, snapshots.KindActive, key, parent, opts...)
	if err != nil {
		return nil, err
	}

	snd := filepath.Join(o.cfg.FileSystemRoot, "sfs", sn.ID)
	if err := os.MkdirAll(snd, 0755); err != nil {
		return nil, err
	}

	if ir, hasIr := o.receiver[targetImage]; hasIr {
		mnt = ir.GetLayerMounts()
	} else {
		/*
			rc, headerSize, err := o.remote.FetchWithString(sourceImage, targetImage)
			if err != nil {
				return nil, err
			}

			nir, err := starlightfs.NewReceiver(ctx, o.layerStore, rc, headerSize, sn.ID, func() {
				o.extractionCompleted(targetImage)
			})
			if err != nil {
				return nil, err
			}

			nir.ExtractFiles() //async call

			o.receiver[targetImage] = nir
			mnt = nir.GetLayerMounts()

		*/
		fmt.Println(sourceImage)
	}

	return mnt, nil
}

func (o *snapshotter) extractionCompleted(targetImage string) {

}

func (o *snapshotter) createContainer(ctx context.Context, key, parent, _key, _parent string, config *snapshots.Info, mnt []mount.Mount, opts ...snapshots.Opt) ([]mount.Mount, error) {
	parentBase := path.Base(_parent)
	if strings.HasPrefix(parentBase, "accelerated(") && strings.HasSuffix(parentBase, ")") {
		acceleratedImage := strings.TrimSuffix(strings.TrimPrefix(parentBase, "accelerated("), ")")
		if ir, hasIr := o.receiver[acceleratedImage]; hasIr {
			// snapshot
			sn, err := storage.CreateSnapshot(ctx, snapshots.KindActive, key, parent, opts...)
			if err != nil {
				return nil, err
			}

			// optimize
			optimize := false
			optimizeGroup := ""
			if val, ok := config.Labels[util.OptimizeLabel]; ok && val == "True" {
				optimize = true
			}
			if val, ok := config.Labels[util.OptimizeGroupLabel]; ok {
				optimizeGroup = val
			}

			// fs instance
			fsi, err := ir.NewFsInstance(
				config.Labels[util.ImageNameLabel],
				config.Labels[util.ImageTagLabel],
				path.Join(o.cfg.FileSystemRoot, "sfs", sn.ID),
				optimize,
				optimizeGroup,
			)

			if err != nil {
				return nil, err
			}
			o.fsMap[path.Base(_key)] = fsi

			// mounting point
			mp := filepath.Join(o.cfg.FileSystemRoot, "sfs", sn.ID, "m")
			if err := os.MkdirAll(mp, 0755); err != nil {
				return nil, err
			}

			// fuse server
			fsOpts := fusefs.Options{}
			fsOpts.Debug = o.fsTrace

			fsServer, err := fsi.NewFuseServer(mp, &fsOpts, fsOpts.Debug)
			if err != nil {
				return nil, err
			}
			go fsServer.Serve()
			if err := fsServer.WaitMount(); err != nil {
				return nil, err
			}

			// mounting point
			mountingPoint := fsi.GetMountPoint()
			mnt = []mount.Mount{{
				Type:   "bind",
				Source: mountingPoint,
				Options: []string{
					"rw",
					"rbind",
				},
			}}
			log.G(ctx).WithField("mnt", mnt).Info("fs mounted")

		} else {
			return nil, util.ErrTocUnknown
		}
	}
	return mnt, nil
}

func (o *snapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	//o.imageReadersMux.Lock()
	//defer o.imageReadersMux.Unlock()
	var config snapshots.Info
	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return nil, err
		}
	}
	log.G(ctx).WithFields(logrus.Fields{
		"labels": config.Labels,
		"key":    key,
		"parent": parent,
	}).Info("prepare")

	// =============================================================================================== //
	ctx, t, err := o.ms.TransactionContext(ctx, true)
	if err != nil {
		return nil, err
	}
	defer t.Rollback()

	prepareWorker := true
	if _, hasImageName := config.Labels[util.ImageNameLabel]; !hasImageName {
		prepareWorker = false
	}
	if _, hasImageTag := config.Labels[util.ImageTagLabel]; !hasImageTag {
		prepareWorker = false
	}

	// remove the 6-digit random tag at the end of the command
	_key := key[:len(key)-7]
	_parent := ""
	if parent != "" {
		_parent = parent[:len(parent)-7]
	}

	// this method shares with two commands
	var mnt []mount.Mount
	if !prepareWorker {
		// Step 1 - image pull
		if mnt, err = o.pullImage(ctx, key, parent, _key, _parent, mnt, opts...); err != nil {
			return nil, err
		}
	} else {
		// Step 2 - container create
		if mnt, err = o.createContainer(ctx, key, parent, _key, _parent, &config, mnt, opts...); err != nil {
			return nil, err
		}
	}

	if err := t.Commit(); err != nil {
		return nil, errors.Wrap(err, "commit failed")
	}

	return mnt, nil
	// =============================================================================================== //
}

func (o *snapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	log.G(ctx).WithField("key", key).WithField("parent", parent).Info("view")

	return o.Prepare(ctx, key, parent, opts...)
}

// Mounts returns the mounts for the transaction identified by key. Can be
// called on an read-write or readonly transaction.
//
// This can be used to recover mounts after calling View or Prepare.
func (o *snapshotter) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {
	ctx, t, err := o.ms.TransactionContext(ctx, false)
	if err != nil {
		return nil, err
	}
	defer t.Rollback()

	s, err := storage.GetSnapshot(ctx, key)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get snapshot mount")
	}
	log.G(ctx).WithField("s", s).WithField("key", key).Info("mounts")

	_key := key[:len(key)-7]
	if fsi, hasFs := o.fsMap[path.Base(_key)]; hasFs {
		// Problem! Looking for a better fix
		// Possible of losing mounting points after task started
		mounting := fsi.GetMountPoint()
		_, _ = ioutil.ReadDir(mounting)
		go func() {
			n := 0
			for {
				_, _ = ioutil.ReadDir(mounting)
				time.Sleep(100 * time.Millisecond)
				n++
				if n > 50 {
					break
				}
			}
		}()

		return []mount.Mount{{
			Type:   "bind",
			Source: mounting,
			Options: []string{
				"rw",
				"rbind",
			},
		}}, nil
	} else {
		return nil, util.ErrMountingPointNotFound
	}
}

func (o *snapshotter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error {
	log.G(ctx).WithField("key", key).WithField("name", name).Info("commit")
	ctx, t, err := o.ms.TransactionContext(ctx, true)
	if err != nil {
		return err
	}
	defer t.Rollback()

	id, _, _, err := storage.GetInfo(ctx, key)
	if err != nil {
		return err
	}

	dir := o.getSnDir(id)

	usage, err := fs.DiskUsage(ctx, dir)
	if err != nil {
		return err
	}

	if _, err := storage.CommitActive(ctx, key, name, snapshots.Usage(usage), opts...); err != nil {
		return errors.Wrap(err, "failed to commit snapshot")
	}
	return t.Commit()
}

// Remove abandons the transaction identified by key. All resources
// associated with the key will be removed.
func (o *snapshotter) Remove(ctx context.Context, key string) (err error) {
	log.G(ctx).WithField("key", key).Info("remove")
	ctx, t, err := o.ms.TransactionContext(ctx, true)
	if err != nil {
		return err
	}
	defer t.Rollback()

	_key := key[:len(key)-7]
	base := path.Base(_key)
	if strings.HasPrefix(base, "worker") {
		if fsi, hasFs := o.fsMap[path.Base(_key)]; hasFs {
			// umount
			err := fsi.Teardown()
			log.G(ctx).WithFields(logrus.Fields{
				"key": key,
				"fs":  fsi.GetMountPoint(),
				"err": err,
			}).Info("teardown")
		}
	}

	if id, _, err := storage.Remove(ctx, key); err != nil {
		return errors.Wrap(err, "failed to remove")
	} else {
		err := os.RemoveAll(o.getSnDir(id))
		log.G(ctx).WithFields(logrus.Fields{
			"key": key,
			"id":  id,
			"err": err,
		}).Info("removed")
	}
	if err = t.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit")
	}

	return nil
}
