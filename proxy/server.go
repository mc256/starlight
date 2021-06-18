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
	"context"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/merger"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
	"net/http"
	"strings"
	"sync"
)

type transition struct {
	tagFrom []*util.ImageRef
	tagTo   []*util.ImageRef
}

type StarlightProxyServer struct {
	http.Server

	ctx      context.Context
	database *bolt.DB

	containerRegistry string

	ib *DeltaBundleBuilder
}

func (a *StarlightProxyServer) getDeltaImage(w http.ResponseWriter, req *http.Request, from string, to string) error {
	// Parse Image Reference
	t := &transition{
		tagFrom: make([]*util.ImageRef, 0),
		tagTo:   make([]*util.ImageRef, 0),
	}

	var err error
	if from != "_" {
		if t.tagFrom, err = util.NewImageRef(from); err != nil {
			return err
		}
	}
	if t.tagTo, err = util.NewImageRef(to); err != nil {
		return err
	}

	// Load Optimized Merged Image Collections
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

	/*
		buf := bytes.NewBuffer(make([]byte, 0))
		headerSize, contentLength, err := ib.WriteHeader(buf, false)
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

		if err = ib.WriteBody(w); err != nil {
			log.G(a.ctx).WithField("err", err).Error("write body error")
			return nil
		}
	*/

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

func (a *StarlightProxyServer) getOptimize(w http.ResponseWriter, req *http.Request, group string) error {
	// TODO: receive optimized information.
	//_, _ = io.Copy(os.Stdout, req.Body)

	header := w.Header()
	header.Set("Content-Type", "text/plain")
	header.Set("Starlight-Version", util.Version)
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "Optimize: %s \n", group)
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
	case len(params) == 4 && params[0] == "optimize":
		err = a.getOptimize(w, req, params[1])
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
		ib:                NewBuilder(ctx, registry),
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
