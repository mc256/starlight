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
	"github.com/mc256/starlight/merger"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
	"io"
	"net/http"
	"strings"
	"sync"
)

const (
	SERVER_VERSION = "1.0.0"
)

var (
	containerRegistry = "http://container-worker.momoko:5000"
)

type transition struct {
	tagFrom string
	tagTo   string
}

type AcceleratorServer struct {
	http.Server

	ctx      context.Context
	database *bolt.DB
}

func (a *AcceleratorServer) getDeltaImage(w http.ResponseWriter, req *http.Request, from string, to string) error {

	fromImages := strings.Split(from, ",")
	toImages := strings.Split(to, ",")

	pool := make(map[string]*transition, len(toImages))
	for _, f := range fromImages {
		if f == "_" {
			break
		}
		arr := strings.Split(f, ":")
		if len(arr) != 2 || strings.Trim(arr[0], " ") == "" || strings.Trim(arr[1], " ") == "" {
			return util.ErrWrongImageFormat
		}
		pool[arr[0]] = &transition{
			tagFrom: arr[1],
		}
	}

	for _, t := range toImages {
		arr := strings.Split(t, ":")
		if len(arr) != 2 || strings.Trim(arr[0], " ") == "" || strings.Trim(arr[1], " ") == "" {
			return util.ErrWrongImageFormat
		}
		te, ok := pool[arr[0]]
		if ok {
			te.tagTo = arr[1]
		} else {
			pool[arr[0]] = &transition{
				tagFrom: "",
				tagTo:   arr[1],
			}
		}
	}

	c := merger.NewConsolidator(a.ctx)

	for imageName, t := range pool {
		m1 := merger.NewOverlayBuilder(a.ctx, a.database)
		m2 := merger.NewOverlayBuilder(a.ctx, a.database)

		if t.tagFrom != "" {
			err := m1.AddImage(imageName, t.tagFrom)
			if err != nil {
				return err
			}
		}
		if t.tagTo != "" {
			err := m2.AddImage(imageName, t.tagTo)
			if err != nil {
				return err
			}
			d := merger.GetDelta(a.ctx, m1, m2)
			err = c.AddDelta(d)
		}
	}

	ib, err := NewPreloadImageBuilder(a.ctx, c, containerRegistry)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(make([]byte, 0))
	headerSize, contentLength, err := ib.WriteHeader(buf)
	if err != nil {
		log.G(a.ctx).WithField("err", err).Error("write header cache")
		return nil
	}

	header := w.Header()
	header.Set("Content-Type", "application/octet-stream")
	header.Set("Content-Length", fmt.Sprintf("%d", contentLength))
	header.Set("Accelerator-Header-Size", fmt.Sprintf("%d", headerSize))
	header.Set("Accelerator-Version", SERVER_VERSION)
	header.Set("Content-Disposition", `attachment; filename="accelerated-image.img"`)
	w.WriteHeader(http.StatusOK)

	if n, err := io.CopyN(w, buf, headerSize); err != nil || n != headerSize {
		log.G(a.ctx).WithField("err", err).Error("write header error")
		return nil
	}

	if err = ib.WriteBody(w); err != nil {
		log.G(a.ctx).WithField("err", err).Error("write body error")
		return nil
	}

	return nil
}

func (a *AcceleratorServer) getPrepared(w http.ResponseWriter, req *http.Request, image string) error {
	arr := strings.Split(strings.Trim(image, ""), ":")
	if len(arr) != 2 || arr[0] == "" || arr[1] == "" {
		return util.ErrWrongImageFormat
	}

	err := CacheToc(a.ctx, a.database, arr[0], arr[1], containerRegistry)
	if err != nil {
		return err
	}

	header := w.Header()
	header.Set("Content-Type", "text/plain")
	header.Set("Accelerator-Version", SERVER_VERSION)
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "Cached TOC: %s\n", image)
	return nil
}

func (a *AcceleratorServer) getDefault(w http.ResponseWriter, req *http.Request) {
	_, _ = fmt.Fprint(w, "Accelerator Server!\n")
}

func (a *AcceleratorServer) rootFunc(w http.ResponseWriter, req *http.Request) {
	params := strings.Split(strings.Trim(req.RequestURI, "/"), "/")
	log.G(a.ctx).WithFields(logrus.Fields{
		"remote": req.RemoteAddr,
		"params": params,
	}).Info("request received")
	var err error
	switch {
	case len(params) == 4 && params[0] == "from" && params[2] == "to":
		err = a.getDeltaImage(w, req, params[1], params[3])
		break
	case len(params) == 2 && params[0] == "prepare":
		err = a.getPrepared(w, req, params[1])
		break
	default:
		a.getDefault(w, req)
	}
	if err != nil {
		header := w.Header()
		header.Set("Content-Type", "text/plain")
		header.Set("Accelerator-Version", SERVER_VERSION)
		w.WriteHeader(http.StatusInternalServerError)

		_, _ = fmt.Fprintf(w, "Opoos! Something went wrong: \n\n%s\n", err)
	} else {
		log.G(a.ctx).WithFields(logrus.Fields{
			"remote": req.RemoteAddr,
			"params": params,
		}).Info("request sent")
	}
}

func NewServer(registry, logLevel string, wg *sync.WaitGroup) *AcceleratorServer {
	ctx := util.ConfigLoggerWithLevel(logLevel)

	log.G(ctx).WithFields(logrus.Fields{
		"registry":  registry,
		"log-level": logLevel,
	}).Info("accelerator-server")

	if registry != "" {
		containerRegistry = registry
	}

	server := &AcceleratorServer{
		Server: http.Server{
			Addr: ":8090",
		},
		database: util.OpenDatabase(ctx),
		ctx:      ctx,
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
