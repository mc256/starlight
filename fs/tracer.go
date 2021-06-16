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
	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"sort"
	"sync"
	"time"
)

//////////////////////////
type TraceItem struct {
	FileName string        `json:"f"`
	Access   time.Duration `json:"a"`
	Wait     time.Duration `json:"w"`
}

//////////////////////////
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

//////////////////////////
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

//////////////////////////
type Tracer struct {
	// label could be the name of the application or the workload.
	// Different workload might have
	OptimizeGroup string        `json:"group"`
	Image         util.ImageRef `json:"image"`
	StartTime     time.Time     `json:"start"`
	logPath       string
	fh            *os.File
	mtx           *sync.Mutex
	Seq           []*TraceItem `json:"seq"`
}

func (t *Tracer) Log(fileName string, accessTime, completeTime time.Time) {
	item := TraceItem{
		FileName: fileName,
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

//////////////////////////
func NewTracer(optimizeGroup, imageName, imageTag string) (*Tracer, error) {
	rand.Seed(time.Now().UnixNano())

	r := rand.Intn(99999)
	err := os.MkdirAll(path.Join(os.TempDir(), "starlight-optimizer"), 0775)
	if err != nil {
		return nil, err
	}
	logPath := path.Join(os.TempDir(), "starlight-optimizer", fmt.Sprintf("%05d.log", r))

	fh, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}

	return &Tracer{
		OptimizeGroup: optimizeGroup,
		StartTime:     time.Now(),
		Image: util.ImageRef{
			ImageName: imageName,
			ImageTag:  imageTag,
		},
		logPath: logPath,
		fh:      fh,
		mtx:     &sync.Mutex{},
		Seq:     make([]*TraceItem, 0),
	}, nil
}

//////////////////////////
type OptimizedTraceItem struct {
	TraceItem
	Rank        int `json:"r"`
	SourceImage int `json:"s"`
}

type OptimizedGroup struct {
	History    []*OptimizedTraceItem `json:"h"`
	Images     []*util.ImageRef      `json:"i"`
	standalone bool
}

type TraceCollection struct {
	tracerMap map[string][]Tracer
	groups    []*OptimizedGroup
}

func LoadTraceCollection(ctx context.Context, p string) (*TraceCollection, error) {
	pp := path.Join(p, "starlight-optimizer")
	files, err := ioutil.ReadDir(pp)
	if err != nil {
		return nil, err
	}

	tc := &TraceCollection{
		tracerMap: make(map[string][]Tracer),
		groups:    make([]*OptimizedGroup, 0),
	}

	for _, f := range files {
		if path.Ext(f.Name()) == ".log" {
			buf, err := ioutil.ReadFile(path.Join(pp, f.Name()))
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
					History:    make([]*OptimizedTraceItem, 0),
					Images:     make([]*util.ImageRef, 0),
					standalone: true,
				}
				tc.groups = append(tc.groups, g)

				visited := make(map[string]bool)

				g.Images = append(g.Images, &trace.Image)
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
				History:    make([]*OptimizedTraceItem, 0),
				Images:     make([]*util.ImageRef, 0),
				standalone: false,
			}
			tc.groups = append(tc.groups, g)

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
				g.Images = append(g.Images, &util.ImageRef{
					ImageName: trace.Image.ImageName,
					ImageTag:  trace.Image.ImageTag,
				})
			}

			sort.Sort(util.ByImageName(g.Images))
			lookupMap := make(map[string]int, len(g.Images))
			for i, img := range g.Images {
				lookupMap[img.String()] = i
			}

			// Populate history from multiple traces
			for _, trace := range arr {
				traceOffset := trace.StartTime.Sub(starTime)
				visited := make(map[string]bool)

				idx := lookupMap[trace.Image.String()]

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
