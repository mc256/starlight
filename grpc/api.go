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
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mc256/starlight/proxy"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
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

func (a *StarlightProxy) Ping() error {
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
		return err
	}

	response, err := ioutil.ReadAll(resp.Body)
	version := resp.Header.Get("Starlight-Version")

	var r proxy.ApiResponse
	if err = json.Unmarshal(response, &r); err != nil {
		log.G(a.ctx).WithFields(logrus.Fields{
			"code":     fmt.Sprintf("%d", resp.StatusCode),
			"version":  version,
			"response": strings.TrimSpace(string(response)),
		}).WithError(err).Error("unknown response error")
		return nil
	}

	if resp.StatusCode != 200 && r.Message != "Starlight Proxy" {
		log.G(a.ctx).WithFields(logrus.Fields{
			"code":     fmt.Sprintf("%d", resp.StatusCode),
			"version":  version,
			"response": strings.TrimSpace(string(response)),
		}).Error("server error")
		return nil
	}

	log.G(a.ctx).WithFields(logrus.Fields{
		"code":    200,
		"version": version,
		"rtt":     time.Now().Sub(t).Milliseconds(),
		"unit":    "ms",
	}).Info("server is okay")
	return nil
}

func (a *StarlightProxy) Notify(ref name.Reference) error {
	u := url.URL{
		Scheme: a.protocol,
		Host:   a.serverAddress,
		Path:   path.Join("starlight"),
	}
	q := u.Query()
	q.Set("ref", ref.String())
	q.Set("action", "notify")
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

func (a *StarlightProxy) Report(ref name.Reference, buffer bytes.Buffer) error {

	u := url.URL{
		Scheme: a.protocol,
		Host:   a.serverAddress,
		Path:   path.Join("starlight"),
	}
	q := u.Query()
	q.Set("ref", ref.String())
	q.Set("action", "report")
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
