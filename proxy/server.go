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

package proxy

import (
	"bytes"
	"context"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mc256/starlight/fs"
	"github.com/mc256/starlight/merger"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
)

type transition struct {
	from name.Reference
	to   name.Reference
}

type StarlightProxyServer struct {
	http.Server

	ctx      context.Context
	database *bolt.DB

	containerRegistry string

	builder *DeltaBundleBuilder
}

func (a *StarlightProxyServer) getDeltaImage(w http.ResponseWriter, req *http.Request, from string, to string) error {
	// Parse Image Reference
	var err error
	t := &transition{from: nil, to: nil}
	if from != "_" {
		t.from, err = name.ParseReference(from, name.WithDefaultRegistry(a.containerRegistry))
		if err != nil {
			return fmt.Errorf("failed to parse source image: %v", err)
		}
	}
	t.to, err = name.ParseReference(to, name.WithDefaultRegistry(a.containerRegistry))
	if err != nil {
		return fmt.Errorf("failed to parse distination image: %v", err)
	}

	// Load Optimized Merged Image Collections
	/*
		var cTo, cFrom *Collection

		if cTo, err = LoadCollection(a.ctx, a.database, t.tagTo); err != nil {
			return err
		}
		if len(t.tagFrom) != 0 {
			if cFrom, err = LoadCollection(a.ctx, a.database, t.tagFrom); err != nil {
				return err
			}
			cTo.Minus(cFrom)
		}

		deltaBundle := cTo.ComposeDeltaBundle()
	*/
	collection := Collection{}
	deltaBundle := collection.ComposeDeltaBundle()
	//////

	buf := bytes.NewBuffer(make([]byte, 0))
	wg := &sync.WaitGroup{}

	headerSize, contentLength, err := a.builder.WriteHeader(buf, deltaBundle, wg, false)
	if err != nil {
		log.G(a.ctx).WithField("err", err).Error("write header cache")
		return nil
	}

	header := w.Header()
	header.Set("Content-Type", "application/octet-stream")
	header.Set("Content-Length", fmt.Sprintf("%d", contentLength))
	header.Set("Starlight-Header-Size", fmt.Sprintf("%d", headerSize))
	header.Set("Starlight-Version", util.Version)
	header.Set("Content-Disposition", `attachment; filename="starlight.img"`)
	w.WriteHeader(http.StatusOK)

	if n, err := io.CopyN(w, buf, headerSize); err != nil || n != headerSize {
		log.G(a.ctx).WithField("err", err).Error("write header error")
		return nil
	}

	if err = a.builder.WriteBody(w, deltaBundle, wg); err != nil {
		log.G(a.ctx).WithField("err", err).Error("write body error")
		return nil
	}

	return nil
}

func (a *StarlightProxyServer) getPrepared(w http.ResponseWriter, req *http.Request, image string) error {
	arr := strings.Split(strings.Trim(image, ""), ":")
	if len(arr) != 2 || arr[0] == "" || arr[1] == "" {
		return util.ErrWrongImageFormat
	}

	err := CacheToc(a.ctx, a.database, arr[0], arr[1], a.containerRegistry)
	if err != nil {
		return err
	}

	ob := merger.NewOverlayBuilder(a.ctx, a.database)
	if err = ob.AddImage(arr[0], arr[1]); err != nil {
		return err
	}
	if err = ob.SaveMergedImage(); err != nil {
		return err
	}

	header := w.Header()
	header.Set("Content-Type", "text/plain")
	header.Set("Starlight-Version", util.Version)
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "Cached TOC: %s\n", image)
	return nil
}

func (a *StarlightProxyServer) postReport(w http.ResponseWriter, req *http.Request) error {
	header := w.Header()
	header.Set("Content-Type", "text/plain")
	header.Set("Starlight-Version", util.Version)
	w.WriteHeader(http.StatusOK)

	buf, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}

	tc, err := fs.NewTraceCollectionFromBuffer(buf)
	if err != nil {
		log.G(a.ctx).WithError(err).Info("cannot parse trace collection")
		return err
	}

	for _, grp := range tc.Groups {
		log.G(a.ctx).WithField("collection", grp.Images)
		fso, err := LoadCollection(a.ctx, a.database, grp.Images)
		if err != nil {
			return err
		}

		fso.AddOptimizeTrace(grp)

		if err := fso.SaveMergedApp(); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(w, "Optimized: %s \n", grp.Images)
	}

	return nil
}

func (a *StarlightProxyServer) getDefault(w http.ResponseWriter, req *http.Request) {
	_, _ = fmt.Fprint(w, "Starlight Proxy OK!\n")
}

func (a *StarlightProxyServer) rootFunc(w http.ResponseWriter, req *http.Request) {
	params := strings.Split(strings.Trim(req.RequestURI, "/"), "/")
	remoteAddr := req.RemoteAddr

	if realIp := req.Header.Get("X-Real-IP"); realIp != "" {
		remoteAddr = realIp
	}
	log.G(a.ctx).WithFields(logrus.Fields{
		"remote": remoteAddr,
		"params": params,
	}).Info("request received")
	var err error
	switch {
	case len(params) == 4 && params[0] == "from" && params[2] == "to":
		err = a.getDeltaImage(w, req, strings.TrimSpace(params[1]), strings.TrimSpace(params[3]))
		break
	case len(params) == 2 && params[0] == "prepare":
		err = a.getPrepared(w, req, params[1])
		break
	case len(params) == 1 && params[0] == "report":
		err = a.postReport(w, req)
		break
	default:
		a.getDefault(w, req)
	}
	if err != nil {
		header := w.Header()
		header.Set("Content-Type", "text/plain")
		header.Set("Starlight-Version", util.Version)
		w.WriteHeader(http.StatusInternalServerError)

		_, _ = fmt.Fprintf(w, "Opoos! Something went wrong: \n\n%s\n", err)
	} else {
		log.G(a.ctx).WithFields(logrus.Fields{
			"remote": remoteAddr,
			"params": params,
		}).Info("request sent")
	}
}

func NewServer(registry, logLevel string, wg *sync.WaitGroup) *StarlightProxyServer {
	ctx := util.ConfigLoggerWithLevel(logLevel)

	log.G(ctx).WithFields(logrus.Fields{
		"registry":  registry,
		"log-level": logLevel,
	}).Info("Starlight Proxy")

	db, err := util.OpenDatabase(ctx, util.DataPath, util.ProxyDbName)
	if err != nil {
		log.G(ctx).WithError(err).Error("open database error")
		return nil
	}

	server := &StarlightProxyServer{
		Server: http.Server{
			Addr: ":8090",
		},
		database:          db,
		ctx:               ctx,
		containerRegistry: registry,
		builder:           NewBuilder(ctx, registry),
	}
	http.HandleFunc("/", server.rootFunc)

	go func() {
		defer wg.Done()
		defer server.database.Close()

		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.G(ctx).WithField("error", err).Error("server exit with error")
		}
	}()

	return server
}
