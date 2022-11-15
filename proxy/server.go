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
	"github.com/mc256/starlight/client/fs"
	"github.com/mc256/starlight/util"
	"github.com/mc256/starlight/util/common"
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
	config *Configuration

	cache      map[string]*common.LayerCache
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

func (a *Server) delta(w http.ResponseWriter, req *http.Request) {
	ip := a.getIpAddress(req)
	q := req.URL.Query()
	log.G(a.ctx).WithFields(logrus.Fields{"action": "delta", "ip": ip}).Info("request received")

	f, t, plt := q.Get("from"), q.Get("to"), q.Get("platform")
	if t == "" || plt == "" {
		a.error(w, req, "missing parameters")
		return
	}

	b, err := NewBuilder(a, f, t, plt)
	if err != nil {
		a.error(w, req, err.Error())
		return
	}

	if err = b.Load(); err != nil {
		a.error(w, req, err.Error())
		return
	}

	if err = b.WriteHeader(w, req); err != nil {
		log.G(a.ctx).WithError(err).Error("failed to write delta image header")
		return
	}
	log.G(a.ctx).WithFields(logrus.Fields{"action": "delta", "ip": ip}).Debug("header sent")

	if err = b.WriteBody(w, req); err != nil {
		log.G(a.ctx).WithError(err).Error("failed to write delta image body")
		return
	}

	log.G(a.ctx).WithFields(logrus.Fields{"action": "delta", "ip": ip}).Debug("response sent")
}

func (a *Server) notify(w http.ResponseWriter, req *http.Request) {
	ip := a.getIpAddress(req)
	q := req.URL.Query()
	log.G(a.ctx).WithFields(logrus.Fields{"action": "notify", "ip": ip}).Info("request received")

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
}

func (a *Server) report(w http.ResponseWriter, req *http.Request) {
	ip := a.getIpAddress(req)
	log.G(a.ctx).WithFields(logrus.Fields{"action": "report", "ip": ip}).Info("request received")

	tc, err := fs.NewTraceCollectionFromBuffer(req.Body)
	if err != nil {
		log.G(a.ctx).WithError(err).Info("cannot parse trace collection")
		a.error(w, req, err.Error())
		return
	}

	arr, err := a.db.UpdateFileRanks(tc)
	if err != nil {
		log.G(a.ctx).WithError(err).Info("cannot update file ranks")
		a.error(w, req, err.Error())
		return
	}

	log.G(a.ctx).
		WithField("ip", ip).
		WithField("layers", arr).
		Info("received traces")

	a.respond(w, req, &ApiResponse{
		Status:  "OK",
		Code:    http.StatusOK,
		Message: "Starlight Proxy",
	})
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

func NewServer(ctx context.Context, wg *sync.WaitGroup, cfg *Configuration) (*Server, error) {

	server := &Server{
		ctx: ctx,
		Server: http.Server{
			Addr: fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort),
		},
		config: cfg,
		cache:  make(map[string]*common.LayerCache),
	}

	// connect database
	if db, err := NewDatabase(ctx, cfg.PostgresConnectionString); err != nil {
		log.G(ctx).Errorf("failed to connect to database: %v\n", err)
	} else {
		server.db = db
	}

	// init database
	i := 0
	for {
		i += 1
		err := server.db.InitDatabase()
		if err != nil {
			if i > 10 {
				log.G(ctx).Errorf("failed to init database: %v\n", err)
				return nil, err
			}
			log.G(ctx).
				WithError(err).
				Errorf("failed to connect to database, retrying in 5 seconds (%d/10)", i)
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}
	log.G(ctx).Debug("database initialized")

	// create router
	http.HandleFunc("/scanner", server.scanner)
	http.HandleFunc("/starlight/delta", server.delta)
	http.HandleFunc("/starlight/notify", server.notify)
	http.HandleFunc("/starlight/report", server.report)
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

	return server, nil
}
