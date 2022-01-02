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

import "io"

type CountWriter struct {
	w     io.Writer
	count int64
}

func (c *CountWriter) Write(p []byte) (n int, err error) {
	n, err = c.w.Write(p)
	c.count += int64(n)
	return
}

func (c *CountWriter) GetWrittenSize() int64 {
	return c.count
}

func NewCountWriter(w io.Writer) (cw *CountWriter) {
	cw = &CountWriter{
		w:     w,
		count: 0,
	}
	return
}
