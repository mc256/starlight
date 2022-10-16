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
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
	"net/http"
	"sync"
	"time"
)

type transition struct {
	from name.Reference
	to   name.Reference
}

type ApiResponse struct {
	Status string `json:"status"`
	Code   int    `json:"code"`

	// Common Response
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`

	// Responses Information
	Extractor *Extractor `json:"extractor,omitempty"`
}

type Server struct {
	http.Server
	ctx context.Context

	db     *Database
	config *ProxyConfiguration

	cache      map[string]*LayerCache
	cacheMutex sync.Mutex
}

func (a *Server) getIpAddress(req *http.Request) string {
	remoteAddr := req.RemoteAddr
	if realIp := req.Header.Get("X-Real-IP"); realIp != "" {
		remoteAddr = realIp
	}
	return remoteAddr
}

func (a *Server) cacheTimeoutValidator() {
	for k, v := range a.cache {
		v.Mutex.Lock()
		// Delete Expired Cache
		if v.UseCounter <= 0 && time.Now().Add(time.Duration(a.config.CacheTimeout)*time.Second).Before(v.LastUsed) {
			delete(a.cache, k)
		}
		v.Mutex.Unlock()
	}
	time.Sleep(time.Second)
}

func (a *Server) root(w http.ResponseWriter, req *http.Request) {
	log.G(a.ctx).WithFields(logrus.Fields{"ip": a.getIpAddress(req)}).Info("root")

	header := w.Header()
	header.Set("Content-Type", "application/json")
	header.Set("Starlight-Version", util.Version)
	w.WriteHeader(http.StatusOK)

	r := ApiResponse{
		Status:  "OK",
		Code:    http.StatusOK,
		Message: "Starlight Proxy",
	}
	b, _ := json.Marshal(r)
	_, _ = w.Write(b)
}

func (a *Server) scanner(w http.ResponseWriter, req *http.Request) {
	log.G(a.ctx).WithFields(logrus.Fields{"ip": a.getIpAddress(req)}).Info("harbor scanner")

	// TODO: implement api hooks
	header := w.Header()
	header.Set("Content-Type", "application/json")
	header.Set("Starlight-Version", util.Version)
	w.WriteHeader(http.StatusNotImplemented)

	r := ApiResponse{
		Status:  "Not Implemented",
		Code:    http.StatusNotImplemented,
		Message: "Starlight Proxy",
	}
	b, _ := json.Marshal(r)
	_, _ = w.Write(b)
}

func (a *Server) starlight(w http.ResponseWriter, req *http.Request) {
	ip := a.getIpAddress(req)
	q := req.URL.Query()
	action := q.Get("action")

	log.G(a.ctx).WithFields(logrus.Fields{"action": action, "ip": ip}).Info("request received")

	switch action {
	case "delta-image":
		f, t := q.Get("from"), q.Get("to")
		if t == "" {
			a.error(w, req, "missing parameters")
			return
		}

		b, err := NewBuilder(a, f, t)
		if err != nil {
			a.error(w, req, err.Error())
			return
		}

		if err = b.WriteHeader(w, req); err != nil {
			log.G(a.ctx).WithError(err).Error("failed to write delta image header")
			return
		}
		if err = b.WriteBody(w, req); err != nil {
			log.G(a.ctx).WithError(err).Error("failed to write delta image body")
			return
		}
		return

	case "notify":
		i := q.Get("ref")
		if i == "" {
			a.error(w, req, "missing parameters")
			return
		}

		extractor, err := NewExtractor(a, i)
		if err != nil {
			log.G(a.ctx).WithError(err).Error("failed to cache ToC")
			a.error(w, req, err.Error())
			return
		}

		res, err := extractor.SaveToC()
		if err != nil {
			log.G(a.ctx).WithError(err).Error("failed to cache ToC")
			a.error(w, req, err.Error())
			return
		}

		log.G(a.ctx).WithField("container", i).Info("cached ToC")
		a.respond(w, req, res)
		return

	case "report-traces":
		i := q.Get("image")
		fmt.Println(i)

		// tc, err := fs.NewTraceCollectionFromBuffer(buf)
		/*
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
		*/

		a.error(w, req, "not implemented yet!")
		return

	default:
		a.error(w, req, "unknown action ('delta-image', 'notify' or 'report-traces' expected)")
	}

	a.error(w, req, "missing parameter")
}

func (a *Server) error(w http.ResponseWriter, req *http.Request, reason string) {
	a.respond(w, req, &ApiResponse{
		Status: "Bad Request",
		Code:   http.StatusBadRequest,
		Error:  reason,
	})
}

func (a *Server) respond(w http.ResponseWriter, req *http.Request, res *ApiResponse) {
	header := w.Header()
	header.Set("Content-Type", "application/json")
	header.Set("Starlight-Version", util.Version)
	w.WriteHeader(res.Code)
	b, _ := json.Marshal(res)
	_, _ = w.Write(b)
}

func (a *Server) healthCheck(w http.ResponseWriter, req *http.Request) {
	log.G(a.ctx).WithFields(logrus.Fields{"ip": a.getIpAddress(req)}).Info("health check")

	header := w.Header()
	header.Set("Content-Type", "application/json")
	header.Set("Starlight-Version", util.Version)
	w.WriteHeader(http.StatusOK)

	r := ApiResponse{
		Status:  "OK",
		Code:    http.StatusOK,
		Message: "Starlight Proxy",
	}
	b, _ := json.Marshal(r)
	_, _ = w.Write(b)
}

func NewServer(ctx context.Context, wg *sync.WaitGroup, cfg *ProxyConfiguration) *Server {

	server := &Server{
		ctx: ctx,
		Server: http.Server{
			Addr: fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort),
		},
		config: cfg,
		cache:  make(map[string]*LayerCache),
	}

	// connect database
	if db, err := NewDatabase(cfg.PostgresConnectionString); err != nil {
		log.G(ctx).Errorf("failed to connect to database: %v\n", err)
	} else {
		server.db = db
	}

	http.HandleFunc("/scanner", server.scanner)
	http.HandleFunc("/starlight", server.starlight)
	http.HandleFunc("/health-check", server.healthCheck)
	http.HandleFunc("/", server.root)

	go func() {
		defer wg.Done()
		defer server.db.Close()

		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.G(ctx).WithField("error", err).Error("server exit with error")
		}
	}()

	go func() {
		for {
			server.cacheTimeoutValidator()
		}
	}()

	return server
}
