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
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/mc256/starlight/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"strings"
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

type LayerCache struct {
	buffer *bytes.Buffer

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

func (lc *LayerCache) Load(server *Server) (err error) {
	defer lc.SetReady(err)

	var l v1.Layer
	l, err = remote.Layer(lc.digest, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return err
	}
	var rc io.ReadCloser
	rc, err = l.Compressed()
	if err != nil {
		log.G(server.ctx).WithField("layer", lc.String()).Error(errors.Wrapf(err, "failed to load layer"))
		return err
	}

	var n int64
	n, err = io.Copy(lc.buffer, rc)
	if err != nil {
		log.G(server.ctx).WithField("layer", lc.String()).Error(errors.Wrapf(err, "failed to load layer"))
		return err
	}
	if n != lc.size {
		err = fmt.Errorf("size unmatch expected %d, but got %d", lc.size, n)
		log.G(server.ctx).WithField("layer", lc.String()).Error(errors.Wrapf(err, "failed to load layer"))
		return err
	}

	return nil
}

func NewLayerCache(layer *ImageLayer) *LayerCache {
	return &LayerCache{
		buffer: bytes.NewBuffer([]byte{}),
		Mutex:  sync.Mutex{},
		Ready:  false,

		UseCounter: 0,
		LastUsed:   time.Now(),
		digest:     layer.digest,
		size:       layer.size,
	}
}

/*
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

*/

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

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
	command := strings.Trim(strings.TrimPrefix(req.RequestURI, "/starlight"), "/")
	q := req.URL.Query()
	log.G(a.ctx).WithFields(logrus.Fields{"command": command, "ip": a.getIpAddress(req)}).Info("request received")

	if req.Method == http.MethodGet && strings.HasPrefix(command, "delta-image") {
		// Get Delta Image
		f, t := q.Get("from"), q.Get("to")

		b, err := NewBuilder(a, f, t)
		if err != nil {
			a.error(w, req, err.Error())
		}

		if err = b.WriteHeader(); err != nil {
			a.error(w, req, err.Error())
		}
		if err = b.WriteBody(); err != nil {
			a.error(w, req, err.Error())
		}
		return
	}

	if req.Method == http.MethodPut && strings.HasPrefix(command, "prepare") {
		// Cache ToC
		i := q.Get("image")

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
	}

	if req.Method == http.MethodPost && strings.HasPrefix(command, "report") {
		// Report FS traces
		i := q.Get("image")
		fmt.Println(i)
		a.error(w, req, "not implemented yet!")
		return
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

	http.HandleFunc("/scanner/", server.scanner)
	http.HandleFunc("/starlight/", server.starlight)
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
