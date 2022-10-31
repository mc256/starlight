/*
   file created by Junlin Chen in 2022

*/

package common

import (
	"bytes"
	"context"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
	"io"
	"sync"
	"time"
)

type CacheInterface interface {
	Digest() name.Digest
	Size() int64
}

type LayerCache struct {
	Buffer *io.SectionReader

	Mutex      sync.Mutex
	Ready      bool
	UseCounter int
	LastUsed   time.Time

	subscribers []*chan error

	digest name.Digest
	size   int64
}

func (lc *LayerCache) String() string {
	lc.Mutex.Lock()
	defer lc.Mutex.Unlock()
	return fmt.Sprintf("%s:%v:%02d:%s",
		lc.digest, lc.Ready, lc.UseCounter, lc.LastUsed.Format(time.RFC3339Nano))
}

func (lc *LayerCache) SetReady(err error) {
	lc.Mutex.Lock()
	defer lc.Mutex.Unlock()
	lc.Ready = true
	lc.notify(err)
}

func (lc *LayerCache) notify(err error) {
	for _, s := range lc.subscribers {
		if err != nil {
			*s <- err
		}
		close(*s)
	}
}

func (lc *LayerCache) Subscribe(errChan *chan error) {
	lc.Mutex.Lock()
	defer lc.Mutex.Unlock()
	if lc.Ready {
		close(*errChan)
		return
	}
	lc.subscribers = append(lc.subscribers, errChan)
}

func (lc *LayerCache) Load(ctx context.Context) (err error) {
	defer lc.SetReady(err)

	var l v1.Layer
	l, err = remote.Layer(lc.digest, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return err
	}
	var rc io.ReadCloser
	rc, err = l.Compressed()
	if err != nil {
		log.G(ctx).WithField("layer", lc.String()).Error(errors.Wrapf(err, "failed to load layer"))
		return err
	}

	buf := new(bytes.Buffer)
	var n int64
	n, err = io.Copy(buf, rc)
	if err != nil {
		log.G(ctx).WithField("layer", lc.String()).Error(errors.Wrapf(err, "failed to load layer"))
		return err
	}
	if n != lc.size {
		err = fmt.Errorf("size unmatch expected %d, but got %d", lc.size, n)
		log.G(ctx).WithField("layer", lc.String()).Error(errors.Wrapf(err, "failed to load layer"))
		return err
	}
	rc.Close()

	lc.Buffer = io.NewSectionReader(bytes.NewReader(buf.Bytes()), 0, n)

	return nil
}

func NewLayerCache(layer CacheInterface) *LayerCache {
	return &LayerCache{
		Buffer: nil,
		Mutex:  sync.Mutex{},
		Ready:  false,

		UseCounter: 0,
		LastUsed:   time.Now(),
		digest:     layer.Digest(),
		size:       layer.Size(),
	}
}
