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

package main

import (
	"fmt"
	"github.com/mc256/starlight/proxy"
	"os"
	"sync"
)

func main() {
	if len(os.Args) != 2 && len(os.Args) != 3 {
		fmt.Printf("Usage: %s RegistryURL [Log Level]\n", os.Args[0])
		return
	}

	logLevel := "info"
	if len(os.Args) == 3 {
		logLevel = os.Args[2]
	}

	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)

	_ = proxy.NewServer(os.Args[1], logLevel, httpServerExitDone)

	wait := make(chan interface{})
	<-wait
}
