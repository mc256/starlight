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
	"context"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
)

type StarlightProxy struct {
	ctx context.Context

	protocol      string
	serverAddress string

	client *http.Client
}

func (a *StarlightProxy) Prepare(imageName, imageTag string) error {
	url := fmt.Sprintf("%s://%s", a.protocol, path.Join(a.serverAddress, "prepare", imageName+":"+imageTag))
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		log.G(a.ctx).WithFields(logrus.Fields{
			"code":      fmt.Sprintf("%d", resp.StatusCode),
			"version":   resp.Header.Get("Starlight-Version"),
			"imageName": imageName,
			"imageTag":  imageTag,
		}).Warn("server prepare error")
		return util.ErrUnknownManifest
	}

	log.G(a.ctx).WithFields(logrus.Fields{
		"version":   resp.Header.Get("Starlight-Version"),
		"imageName": imageName,
		"imageTag":  imageTag,
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
