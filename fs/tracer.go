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

package fs

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path"
	"sync"
	"time"
)

type TraceItem struct {
	FileName string        `json:"f"`
	Access   time.Duration `json:"a"`
	Wait     time.Duration `json:"w"`
}

type Tracer struct {
	imageName string
	imageTag  string
	logPath   string
	fh        *os.File
	mtx       *sync.Mutex
}

func (t *Tracer) Log(fileName string, access time.Duration, wait time.Duration) {
	item := TraceItem{
		FileName: fileName,
		Access:   access,
		Wait:     wait,
	}
	b, _ := json.Marshal(&item)

	t.mtx.Lock()
	defer t.mtx.Unlock()

	_, _ = t.fh.Write(b)
	_, _ = t.fh.WriteString("\n")
}

func (t *Tracer) Close() error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.fh.Close()
}

func NewTracer(imageName, imageTag string) (*Tracer, error) {
	rand.Seed(time.Now().Unix())
	r := rand.Intn(99999)
	err := os.MkdirAll(path.Join(os.TempDir(), "starlight-optimizer", imageName, imageTag), 0775)
	if err != nil {
		return nil, err
	}
	logPath := path.Join(os.TempDir(), "starlight-optimizer", imageName, imageTag, fmt.Sprintf("%05d.log", r))

	fh, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}

	return &Tracer{
		imageName: imageName,
		imageTag:  imageTag,
		logPath:   logPath,
		fh:        fh,
		mtx:       &sync.Mutex{},
	}, nil
}
