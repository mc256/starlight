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
	"strings"
	"sync"
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

	// Extractor Response Information
	Extractor *ExtractorResult `json:"extractor,omitempty"`
}

type StarlightProxyServer struct {
	http.Server
	ctx context.Context

	// new variables
	db     *Database
	config *ProxyConfiguration
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

*/

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (a *StarlightProxyServer) root(w http.ResponseWriter, req *http.Request) {
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

func (a *StarlightProxyServer) scanner(w http.ResponseWriter, req *http.Request) {
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

func (a *StarlightProxyServer) starlight(w http.ResponseWriter, req *http.Request) {
	command := strings.Trim(strings.TrimPrefix(req.RequestURI, "/starlight"), "/")
	q := req.URL.Query()
	log.G(a.ctx).WithFields(logrus.Fields{"command": command, "ip": a.getIpAddress(req)}).Info("request received")

	if req.Method == http.MethodGet && strings.HasPrefix(command, "delta-image") {
		// Get Delta Image
		//f, t := q.Get("from"), q.Get("to")

		a.error(w, req, "not implemented yet!")
		return
	}
	if req.Method == http.MethodPut && strings.HasPrefix(command, "prepare") {
		// Cache ToC
		i := q.Get("image")
		if r, e := SaveToC(a, i); e != nil {
			log.G(a.ctx).WithError(e).Error("failed to cache ToC")
			a.error(w, req, e.Error())
		} else {
			log.G(a.ctx).WithField("container", i).Info("cached ToC")
			a.respond(w, req, r)
		}
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

func (a *StarlightProxyServer) error(w http.ResponseWriter, req *http.Request, reason string) {
	a.respond(w, req, &ApiResponse{
		Status: "Bad Request",
		Code:   http.StatusBadRequest,
		Error:  reason,
	})
}

func (a *StarlightProxyServer) respond(w http.ResponseWriter, req *http.Request, res *ApiResponse) {
	header := w.Header()
	header.Set("Content-Type", "application/json")
	header.Set("Starlight-Version", util.Version)
	w.WriteHeader(res.Code)
	b, _ := json.Marshal(res)
	_, _ = w.Write(b)
}

func (a *StarlightProxyServer) healthCheck(w http.ResponseWriter, req *http.Request) {
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

func (a *StarlightProxyServer) getIpAddress(req *http.Request) string {
	remoteAddr := req.RemoteAddr
	if realIp := req.Header.Get("X-Real-IP"); realIp != "" {
		remoteAddr = realIp
	}
	return remoteAddr
}

func NewServer(ctx context.Context, wg *sync.WaitGroup, cfg *ProxyConfiguration) *StarlightProxyServer {

	server := &StarlightProxyServer{
		ctx: ctx,
		Server: http.Server{
			Addr: fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort),
		},
		config: cfg,
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

	return server
}
