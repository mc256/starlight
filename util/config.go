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

package util

import (
	"os"
	"path/filepath"
)

const (
	ImageNameLabel     = "containerd.io/snapshot/remote/starlight/imageName.label"
	ImageTagLabel      = "containerd.io/snapshot/remote/starlight/imageTag.label"
	OptimizeLabel      = "containerd.io/snapshot/remote/starlight/optimize.label"
	OptimizeGroupLabel = "containerd.io/snapshot/remote/starlight/optimizeGroup.label"
	ImageMediaType     = "application/vnd.mc256.starlight.toc.json"
	UserRwLayerText    = "containerd.io/layer/user-rw-layer"

	SnapshotterDbName = "metadata.db"
	ProxyDbName       = "toc.db"
	DataPath          = "data/"

	StarlightTOCDigestAnnotation       = "containerd.io/snapshot/remote/starlight/toc.digest"
	StarlightTOCCreationTimeAnnotation = "containerd.io/snapshot/remote/starlight/toc.timestamp"

	// Switch to false in `Makefile` when build for production environment
	production = false

	ProjectIdentifier = "module github.com/mc256/starlight"
)

// FindProjectRoot returns the root directory of the Git project if exists.
// otherwise, it returns os.Getwd().
// To identify whether a directory is a root directory, it check the `go.mod` file
// please making sure the first line of the go mode file is:
// ```
// module github.com/mc256/starlight
// ```
func FindProjectRoot() string {
	r, err := os.Getwd()
	if err != nil {
		return ""
	}
	p := r

	for p != "/" && len(p) != 0 {
		f, _ := os.OpenFile(filepath.Join(p, "go.mod"), os.O_RDONLY, 0644)
		b := make([]byte, len(ProjectIdentifier))
		_, _ = f.Read(b)
		if string(b) == ProjectIdentifier {
			return p
		}
		p = filepath.Join(p, "../")
	}
	return r
}

// GetEtcConfigPath return a path to the configuration json
func GetEtcConfigPath() string {
	if production {
		return "/etc/starlight/"
	} else {
		return filepath.Join(FindProjectRoot(), "sandbox", "etc", "starlight")
	}
}
