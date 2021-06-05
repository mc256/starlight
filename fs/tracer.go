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
	"sort"
	"sync"
	"time"
)

type TraceItem struct {
	FileName string        `json:"f"`
	Access   time.Duration `json:"a"`
	Wait     time.Duration `json:"w"`
}

type ByAccessTime []*TraceItem

func (b ByAccessTime) Len() int {
	return len(b)
}

func (b ByAccessTime) Less(i, j int) bool {
	// should return true is i has higher priority
	return b[i].Access < b[j].Access
}
func (b ByAccessTime) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

type Tracer struct {
	// label could be the name of the application or the workload.
	// Different workload might have
	OptimizeGroup string    `json:"group"`
	ImageName     string    `json:"name"`
	ImageTag      string    `json:"tag"`
	StartTime     time.Time `json:"start"`
	logPath       string
	fh            *os.File
	mtx           *sync.Mutex
	Swq           []*TraceItem `json:"seq"`
}

func (t *Tracer) Log(fileName string, accessTime, completeTime time.Time) {
	item := TraceItem{
		FileName: fileName,
		Access:   accessTime.Sub(t.StartTime),
		Wait:     completeTime.Sub(accessTime),
	}

	t.mtx.Lock()
	defer t.mtx.Unlock()

	t.Swq = append(t.Swq, &item)
}

func (t *Tracer) Close() error {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	sort.Sort(ByAccessTime(t.Swq))

	b, _ := json.Marshal(t)
	_, _ = t.fh.Write(b)

	return t.fh.Close()
}

func NewTracer(optimizeGroup, imageName, imageTag string) (*Tracer, error) {
	rand.Seed(time.Now().Unix())
	r := rand.Intn(99999)
	err := os.MkdirAll(path.Join(os.TempDir(), "starlight-optimizer", optimizeGroup, imageName, imageTag), 0775)
	if err != nil {
		return nil, err
	}
	logPath := path.Join(os.TempDir(), "starlight-optimizer", optimizeGroup, imageName, imageTag, fmt.Sprintf("%05d.log", r))

	fh, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}

	return &Tracer{
		OptimizeGroup: optimizeGroup,
		StartTime:     time.Now(),
		ImageName:     imageName,
		ImageTag:      imageTag,
		logPath:       logPath,
		fh:            fh,
		mtx:           &sync.Mutex{},
	}, nil
}
