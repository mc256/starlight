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
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
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

	response, err := ioutil.ReadAll(resp.Body)
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

	rtt := time.Now().Sub(t).Milliseconds()

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

func (a *StarlightProxy) Notify(ref name.Reference) error {
	u := url.URL{
		Scheme: a.protocol,
		Host:   a.serverAddress,
		Path:   path.Join("starlight", "notify"),
	}
	q := u.Query()
	q.Set("ref", ref.String())
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

	response, err := ioutil.ReadAll(resp.Body)
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

func (a *StarlightProxy) DeltaImage(from, to, platform string) (
	reader io.ReadCloser,
	manifestSize, configSize, starlightHeaderSize int64,
	digest, slDigest string,
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
		return nil, 0, 0, 0, "", "", err
	}

	version := resp.Header.Get("Starlight-Version")
	if resp.StatusCode == 400 {
		response, _ := ioutil.ReadAll(resp.Body)
		e := strings.TrimSpace(string(response))
		log.G(a.ctx).
			WithFields(logrus.Fields{
				"code":     fmt.Sprintf("%d", resp.StatusCode),
				"version":  version,
				"response": e,
			}).
			Error("server error")
		return nil, 0, 0, 0, "", "", fmt.Errorf(e)
	}
	if resp.StatusCode != 200 || version == "" {
		response, err := ioutil.ReadAll(resp.Body)
		log.G(a.ctx).
			WithFields(logrus.Fields{
				"code":     fmt.Sprintf("%d", resp.StatusCode),
				"version":  version,
				"response": strings.TrimSpace(string(response)),
			}).
			WithError(err).
			Error("server error")
		return nil, 0, 0, 0, "", "", err
	}

	log.G(a.ctx).WithFields(logrus.Fields{
		"code":     200,
		"version":  version,
		"from":     from,
		"to":       to,
		"platform": platform,
	}).Info("reading delta image")

	manifestSize, err = parseNumber("Manifest-Size", resp.Header.Get("Manifest-Size"))
	if err != nil {
		return nil, 0, 0, 0, "", "", err
	}

	configSize, err = parseNumber("Config-Size", resp.Header.Get("Config-Size"))
	if err != nil {
		return nil, 0, 0, 0, "", "", err
	}

	starlightHeaderSize, err = parseNumber("Starlight-Header-Size", resp.Header.Get("Starlight-Header-Size"))
	if err != nil {
		return nil, 0, 0, 0, "", "", err
	}

	digest = resp.Header.Get("Digest")
	if digest == "" {
		return nil, 0, 0, 0, "", "", fmt.Errorf("header Digest not found")
	}

	slDigest = resp.Header.Get("Starlight-Digest")
	if slDigest == "" {
		return nil, 0, 0, 0, "", "", fmt.Errorf("header Starlight-Digest not found")
	}

	return resp.Body, manifestSize, configSize, starlightHeaderSize, digest, slDigest, nil
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

	response, err := ioutil.ReadAll(resp.Body)
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
