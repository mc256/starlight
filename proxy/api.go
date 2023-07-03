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
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mc256/starlight/util/common"
	"github.com/sirupsen/logrus"
)

type StarlightProxy struct {
	ctx context.Context

	protocol      string
	serverAddress string

	client *http.Client

	auth url.Userinfo
}

func (a *StarlightProxy) Ping() (int64, string, string, error) {
	u := url.URL{
		Scheme: a.protocol,
		Host:   a.serverAddress,
		Path:   "",
	}
	q := u.Query()
	t := time.Now()
	q.Set("t", t.Format(time.RFC3339Nano))
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(a.ctx, "POST", u.String(), nil)
	if pwd, isSet := a.auth.Password(); isSet {
		req.SetBasicAuth(a.auth.Username(), pwd)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return -1, "", "", err
	}

	response, err := io.ReadAll(resp.Body)
	version := resp.Header.Get("Starlight-Version")

	var r ApiResponse
	if err = json.Unmarshal(response, &r); err != nil {
		log.G(a.ctx).WithFields(logrus.Fields{
			"code":     fmt.Sprintf("%d", resp.StatusCode),
			"version":  version,
			"response": strings.TrimSpace(string(response)),
		}).WithError(err).Error("unknown response error")
		return -1, "", "", err
	}

	if resp.StatusCode != 200 && r.Message != "Starlight Proxy" {
		log.G(a.ctx).WithFields(logrus.Fields{
			"code":     fmt.Sprintf("%d", resp.StatusCode),
			"version":  version,
			"response": strings.TrimSpace(string(response)),
		}).Error("server error")
		return -1, "", "", err
	}

	rtt := time.Since(t).Milliseconds()

	log.G(a.ctx).WithFields(logrus.Fields{
		"code":    200,
		"version": version,
		"rtt":     rtt,
		"unit":    "ms",

		"scheme": a.protocol,
		"host":   a.serverAddress,
	}).Info("server is okay")
	return rtt, a.protocol, a.serverAddress, nil
}

func (a *StarlightProxy) Notify(ref name.Reference, insecure bool) error {
	u := url.URL{
		Scheme: a.protocol,
		Host:   a.serverAddress,
		Path:   path.Join("starlight", "notify"),
	}
	q := u.Query()
	q.Set("ref", ref.String())
	if insecure {
		q.Set("insecure", "true")
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(a.ctx, "POST", u.String(), nil)
	if pwd, isSet := a.auth.Password(); isSet {
		req.SetBasicAuth(a.auth.Username(), pwd)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}

	response, err := io.ReadAll(resp.Body)
	version := resp.Header.Get("Starlight-Version")

	if resp.StatusCode != 200 {
		log.G(a.ctx).WithFields(logrus.Fields{
			"code":     fmt.Sprintf("%d", resp.StatusCode),
			"version":  version,
			"ref":      ref.String(),
			"response": strings.TrimSpace(string(response)),
		}).Error("server error")
		return nil
	}

	log.G(a.ctx).WithFields(logrus.Fields{
		"code":     200,
		"version":  version,
		"ref":      ref.String(),
		"response": strings.TrimSpace(string(response)),
	}).Info("server prepared")
	return nil
}

func parseNumber(k, s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("header %s not found", k)
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("header %s expect a number but get %s", k, s)
	}
	return n, nil
}

func getHeaderInt64(h *http.Header, k string) (int64, error) {
	return parseNumber(k, h.Get(k))
}

func (a *StarlightProxy) DeltaImage(from, to, platform string, disableEarlyStart bool) (
	reader io.ReadCloser,
	metadata *common.DeltaImageMetadata,
	err error) {
	u := url.URL{
		Scheme: a.protocol,
		Host:   a.serverAddress,
		Path:   path.Join("starlight", "delta"),
	}
	q := u.Query()
	q.Set("from", from)
	q.Set("to", to)
	q.Set("platform", platform)
	if disableEarlyStart {
		// if early start is disabled, we should disable sorting on the proxy side as well
		q.Set("disableSorting", "true")
	}
	u.RawQuery = q.Encode()

	log.G(a.ctx).WithFields(logrus.Fields{
		"from":     from,
		"to":       to,
		"platform": platform,
	}).Info("request delta image")

	var req *http.Request
	req, err = http.NewRequestWithContext(a.ctx, "GET", u.String(), nil)
	if pwd, isSet := a.auth.Password(); isSet {
		req.SetBasicAuth(a.auth.Username(), pwd)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	version := resp.Header.Get("Starlight-Version")
	if resp.StatusCode == 400 {
		response, _ := io.ReadAll(resp.Body)
		e := strings.TrimSpace(string(response))
		log.G(a.ctx).
			WithFields(logrus.Fields{
				"code":     fmt.Sprintf("%d", resp.StatusCode),
				"version":  version,
				"response": e,
			}).
			Error("server error")
		return nil, nil, fmt.Errorf(e)
	}
	if resp.StatusCode != 200 || version == "" {
		response, err := io.ReadAll(resp.Body)
		log.G(a.ctx).
			WithFields(logrus.Fields{
				"code":     fmt.Sprintf("%d", resp.StatusCode),
				"version":  version,
				"response": strings.TrimSpace(string(response)),
			}).
			WithError(err).
			Error("server error")
		return nil, nil, err
	}

	log.G(a.ctx).WithFields(logrus.Fields{
		"code":     200,
		"version":  version,
		"from":     from,
		"to":       to,
		"platform": platform,
	}).Info("reading delta image")

	res := &common.DeltaImageMetadata{}

	res.ManifestSize, err = getHeaderInt64(&resp.Header, "Manifest-Size")
	if err != nil {
		return nil, nil, err
	}

	res.ConfigSize, err = getHeaderInt64(&resp.Header, "Config-Size")
	if err != nil {
		return nil, nil, err
	}

	res.StarlightHeaderSize, err = getHeaderInt64(&resp.Header, "Starlight-Header-Size")
	if err != nil {
		return nil, nil, err
	}

	res.ContentLength, err = getHeaderInt64(&resp.Header, "Content-Length")
	if err != nil {
		return nil, nil, err
	}

	res.Digest = resp.Header.Get("Digest")
	if res.Digest == "" {
		return nil, nil, fmt.Errorf("header Digest not found")
	}

	res.StarlightDigest = resp.Header.Get("Starlight-Digest")
	if res.StarlightDigest == "" {
		return nil, nil, fmt.Errorf("header Starlight-Digest not found")
	}

	return resp.Body, res, nil
}

func (a *StarlightProxy) Report(body io.Reader) error {
	u := url.URL{
		Scheme: a.protocol,
		Host:   a.serverAddress,
		Path:   path.Join("starlight", "report"),
	}
	req, err := http.NewRequestWithContext(a.ctx, "POST", u.String(), body)
	if pwd, isSet := a.auth.Password(); isSet {
		req.SetBasicAuth(a.auth.Username(), pwd)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}

	response, err := io.ReadAll(resp.Body)
	version := resp.Header.Get("Starlight-Version")

	if resp.StatusCode != 200 {
		log.G(a.ctx).WithFields(logrus.Fields{
			"code":     fmt.Sprintf("%d", resp.StatusCode),
			"version":  version,
			"response": strings.TrimSpace(string(response)),
		}).Error("server error")
		return nil
	}

	log.G(a.ctx).WithFields(logrus.Fields{
		"code":     200,
		"version":  version,
		"response": strings.TrimSpace(string(response)),
	}).Info("server prepared")
	return nil
}

func (a *StarlightProxy) SetAuth(username, password string) {
	a.auth = *url.UserPassword(username, password)
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
