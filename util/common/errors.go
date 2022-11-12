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

package common

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNotImplemented           = errors.New("this feature has not yet been implemented")
	ErrLayerNotFound            = errors.New("cannot find layer")
	ErrMountingPointNotFound    = errors.New("cannot find mounting point")
	ErrNotConsolidated          = errors.New("delta image has not yet been consolidated")
	ErrAlreadyConsolidated      = errors.New("delta image has been consolidated already")
	ErrHashCollision            = errors.New("found two files have the same hash but different size")
	ErrMergedImageNotFound      = errors.New("the requested image has not been merged")
	ErrWrongImageFormat         = errors.New("please use this format <image>:<tag>")
	ErrOrphanNode               = errors.New("an entry node has no parent")
	ErrNoRoPath                 = errors.New("entry does not have path to RO layer")
	ErrImageNotFound            = errors.New("cannot find image")
	ErrNoManager                = errors.New("no manager found")
	ErrUnknownSnapshotParameter = errors.New("snapshots should follow a standard format")
	ErrTocUnknown               = errors.New("please prefetch the delta image")
)

// Aggregate combines a list of errors into a single new error.
func ErrorAggregate(errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		points := make([]string, len(errs)+1)
		points[0] = fmt.Sprintf("%d error(s) occurred:", len(errs))
		for i, err := range errs {
			points[i+1] = fmt.Sprintf("* %s", err)
		}
		return errors.New(strings.Join(points, "\n\t"))
	}
}
