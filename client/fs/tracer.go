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
	"context"
	"encoding/json"
	"fmt"
	"github.com/mc256/starlight/util"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"sync"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/sirupsen/logrus"
)

type TraceItem struct {
	FileName string        `json:"f"`
	Stack    int64         `json:"s"`
	Access   time.Duration `json:"a"`
	Wait     time.Duration `json:"w"`
}

type ByAccessTime []*TraceItem

func (b ByAccessTime) Len() int {
	return len(b)
}

func (b ByAccessTime) Less(i, j int) bool {
	return b[i].Access < b[j].Access
}

func (b ByAccessTime) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

type ByAccessTimeOptimized []*OptimizedTraceItem

func (bo ByAccessTimeOptimized) Len() int {
	return len(bo)
}

func (bo ByAccessTimeOptimized) Less(i, j int) bool {
	return bo[i].Access < bo[j].Access
}
func (bo ByAccessTimeOptimized) Swap(i, j int) {
	bo[i], bo[j] = bo[j], bo[i]
}

type Tracer struct {
	// label could be the name of the application or the workload.
	// Different workload might have
	OptimizeGroup string    `json:"group"`
	Image         string    `json:"image"`
	StartTime     time.Time `json:"start"`
	logPath       string
	fh            *os.File
	mtx           *sync.Mutex
	Seq           []*TraceItem `json:"seq"`
}

func (t *Tracer) Log(fileName string, stack int64, accessTime, completeTime time.Time) {
	item := TraceItem{
		FileName: fileName,
		Stack:    stack,
		Access:   accessTime.Sub(t.StartTime),
		Wait:     completeTime.Sub(accessTime),
	}

	t.mtx.Lock()
	defer t.mtx.Unlock()

	t.Seq = append(t.Seq, &item)
}

func (t *Tracer) Close() error {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	sort.Sort(ByAccessTime(t.Seq))

	b, _ := json.Marshal(t)
	_, _ = t.fh.Write(b)

	return t.fh.Close()
}

func NewTracer(optimizeGroup, digest, outputDir string) (*Tracer, error) {
	err := os.MkdirAll(outputDir, 0775)
	if err != nil {
		return nil, err
	}
	logPath := path.Join(outputDir, fmt.Sprintf("%s.json", util.GetRandomId("trace-")))

	fh, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}

	return &Tracer{
		OptimizeGroup: optimizeGroup,
		StartTime:     time.Now(),
		Image:         digest,
		logPath:       logPath,
		fh:            fh,
		mtx:           &sync.Mutex{},
		Seq:           make([]*TraceItem, 0),
	}, nil
}

// OptimizedTraceItem with ranking
type OptimizedTraceItem struct {
	TraceItem
	Rank        int `json:"r"`
	SourceImage int `json:"s"`
}

func (oti OptimizedTraceItem) Key() string {
	return fmt.Sprintf("%d|%s", oti.SourceImage, oti.TraceItem.FileName)
}

type OptimizedGroup struct {
	History []*OptimizedTraceItem `json:"h"`
	Images  []string              `json:"i"`
}

type TraceCollection struct {
	tracerMap map[string][]Tracer
	Groups    []*OptimizedGroup
}

func (tc TraceCollection) ToJSONBuffer() []byte {
	buf, _ := json.Marshal(tc)
	return buf
}

func NewTraceCollectionFromBuffer(buf io.ReadCloser) (*TraceCollection, error) {
	tc := &TraceCollection{}
	err := json.NewDecoder(buf).Decode(tc)
	if err != nil {
		return nil, err
	}
	return tc, nil
}

func clearTraceCollection(f string) error {
	return os.Remove(f)
}

func NewTraceCollection(ctx context.Context, p string) (*TraceCollection, error) {
	files, err := ioutil.ReadDir(p)
	if err != nil {
		return nil, err
	}

	tc := &TraceCollection{
		tracerMap: make(map[string][]Tracer),
		Groups:    make([]*OptimizedGroup, 0),
	}

	for _, f := range files {
		if path.Ext(f.Name()) == ".json" {
			buf, err := ioutil.ReadFile(path.Join(p, f.Name()))
			if err != nil {
				log.G(ctx).WithField("file", f.Name()).Error("cannot read file")
				continue
			}
			var t Tracer
			err = json.Unmarshal(buf, &t)
			if err != nil {
				log.G(ctx).WithField("file", f.Name()).Error("cannot parse file")
				continue
			}

			log.G(ctx).WithFields(logrus.Fields{
				"file":  f.Name(),
				"group": t.OptimizeGroup,
				"image": t.Image,
			}).Info("parsed file")

			_ = clearTraceCollection(path.Join(p, f.Name()))

			if _, ok := tc.tracerMap[t.OptimizeGroup]; ok {
				tc.tracerMap[t.OptimizeGroup] = append(tc.tracerMap[t.OptimizeGroup], t)
			} else {
				tc.tracerMap[t.OptimizeGroup] = []Tracer{t}
			}
		}
	}

	for key, arr := range tc.tracerMap {
		if key == "default" {
			for _, trace := range arr {
				// Optimized group
				g := &OptimizedGroup{
					History: make([]*OptimizedTraceItem, 0),
					Images:  make([]string, 0),
				}
				tc.Groups = append(tc.Groups, g)

				visited := make(map[string]bool)

				g.Images = append(g.Images, trace.Image)
				for _, t := range trace.Seq {
					if !visited[t.FileName] {
						g.History = append(g.History, &OptimizedTraceItem{
							TraceItem: TraceItem{
								FileName: t.FileName,
								Access:   t.Access,
								Wait:     t.Wait,
							},
							Rank:        0,
							SourceImage: len(g.Images) - 1,
						})
						visited[t.FileName] = true
					}
				}

				for i := 0; i < len(g.History); i++ {
					g.History[i].Rank = i
				}
			}
		} else {
			// Optimized group
			g := &OptimizedGroup{
				History: make([]*OptimizedTraceItem, 0),
				Images:  make([]string, 0),
			}
			tc.Groups = append(tc.Groups, g)

			// Find earliest timestamp
			var starTime time.Time
			for i, trace := range arr {
				if i == 0 {
					starTime = trace.StartTime
				} else {
					if trace.StartTime.Before(starTime) {
						starTime = trace.StartTime
					}
				}
				g.Images = append(g.Images, trace.Image)
			}

			sort.Strings(g.Images)
			lookupMap := make(map[string]int, len(g.Images))
			for i, img := range g.Images {
				lookupMap[img] = i
			}

			// Populate history from multiple traces
			for _, trace := range arr {
				traceOffset := trace.StartTime.Sub(starTime)
				visited := make(map[string]bool)

				idx := lookupMap[trace.Image]

				for _, t := range trace.Seq {
					if !visited[t.FileName] {
						g.History = append(g.History, &OptimizedTraceItem{
							TraceItem: TraceItem{
								FileName: t.FileName,
								Access:   t.Access + traceOffset,
								Wait:     t.Wait,
							},
							Rank:        0,
							SourceImage: idx,
						})
						visited[t.FileName] = true
					}
				}
			}

			// Sort
			sort.Sort(ByAccessTimeOptimized(g.History))
			for i := 0; i < len(g.History); i++ {
				g.History[i].Rank = i
			}
		}
	}

	return tc, nil
}
