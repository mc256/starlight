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

package grpc

import (
	"bytes"
	"context"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type StarlightProxy struct {
	ctx context.Context

	protocol      string
	serverAddress string

	client *http.Client

	auth url.Userinfo
}

func (a *StarlightProxy) Report(buf []byte) error {
	url := fmt.Sprintf("%s://%s", a.protocol, path.Join(a.serverAddress, "report"))
	postBody := bytes.NewBuffer(buf)
	resp, err := http.Post(url, "application/json", postBody)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		log.G(a.ctx).WithFields(logrus.Fields{
			"code":    fmt.Sprintf("%d", resp.StatusCode),
			"version": resp.Header.Get("Starlight-Version"),
		}).Warn("server error")
		return util.ErrUnknownManifest
	}

	log.G(a.ctx).WithFields(logrus.Fields{
		"version": resp.Header.Get("Starlight-Version"),
	}).Info("uploaded filesystem traces")

	resBuf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.G(a.ctx).WithError(err).Error("response body error")
	}

	if resp.StatusCode == 200 {
		log.G(a.ctx).WithFields(logrus.Fields{
			"code":    fmt.Sprintf("%d"),
			"message": strings.TrimSpace(string(resBuf[:])),
			"version": resp.Header.Get("Starlight-Version"),
		}).Info("upload finished")
	} else {
		log.G(a.ctx).WithFields(logrus.Fields{
			"code":    fmt.Sprintf("%d"),
			"message": strings.TrimSpace(string(resBuf[:])),
			"version": resp.Header.Get("Starlight-Version"),
		}).Warn("upload finished")
	}

	return nil
}

func (a *StarlightProxy) Notify(ref name.Reference) error {
	u := url.URL{
		Scheme: a.protocol,
		Host:   a.serverAddress,
		Path:   path.Join("starlight", "notify"),
	}
	q := u.Query()
	q.Set("ref", ref.String())
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(a.ctx, "GET", u.String(), nil)
	if pwd, isSet := a.auth.Password(); isSet {
		req.SetBasicAuth(a.auth.Username(), pwd)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		log.G(a.ctx).WithFields(logrus.Fields{
			"code":    fmt.Sprintf("%d", resp.StatusCode),
			"version": resp.Header.Get("Starlight-Version"),
			"ref":     ref.String(),
		}).Warn("server prepare error")
		return util.ErrUnknownManifest
	}

	log.G(a.ctx).WithFields(logrus.Fields{
		"version": resp.Header.Get("Starlight-Version"),
		"ref":     ref.String(),
	}).Info("server prepared")
	return nil
}

func (a *StarlightProxy) Fetch(have []string, want []string) (io.ReadCloser, int64, error) {
	var fromString string
	if len(have) == 0 {
		fromString = "_"
	} else {
		fromString = strings.Join(have, ",")
	}
	toString := strings.Join(want, ",")

	return a.FetchWithString(fromString, toString)
}

func (a *StarlightProxy) FetchWithString(fromString string, toString string) (io.ReadCloser, int64, error) {
	url := fmt.Sprintf("%s://%s", a.protocol, path.Join(a.serverAddress, "from", fromString, "to", toString))
	//resp, err := http.Get(url)

	req, err := http.NewRequestWithContext(a.ctx, "GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Connection", "Keep-Alive")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, 0, err
	}

	if resp.StatusCode != 200 {
		log.G(a.ctx).WithFields(logrus.Fields{
			"code":    fmt.Sprintf("%d", resp.StatusCode),
			"version": resp.Header.Get("Starlight-Version"),
		}).Warn("server cannot build delta image")
		return nil, 0, util.ErrUnknownManifest
	}

	log.G(a.ctx).WithFields(logrus.Fields{
		"version": resp.Header.Get("Starlight-Version"),
		"from":    fromString,
		"to":      toString,
		"header":  resp.Header.Get("Starlight-Header-Size"),
	}).Info("server prepared delta image")

	headerSize, err := strconv.Atoi(resp.Header.Get("Starlight-Header-Size"))
	if err != nil {
		return nil, 0, err
	}

	return resp.Body, int64(headerSize), nil
}

func NewStarlightProxy(ctx context.Context, protocol, server string) *StarlightProxy {
	return &StarlightProxy{
		ctx:           ctx,
		protocol:      protocol,
		serverAddress: server,
		client: &http.Client{
			Transport:     nil,
			CheckRedirect: nil,
			Jar:           nil,
			Timeout:       0,
		},
	}
}
