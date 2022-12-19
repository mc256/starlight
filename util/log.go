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
	"context"
	"github.com/containerd/containerd/log"
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

func ConfigLogger(ctx context.Context) {
	ConfigLoggerWithLevel(ctx, "info")
}

func ConfigLoggerWithLevel(ctx context.Context, level string) {
	level = strings.ToLower(level)

	log.GetLogger(ctx).Logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.StampNano,
		//ForceColors: true,
		//DisableColors: false,
	})

	l, err := logrus.ParseLevel(level)
	if err != nil {
		l = logrus.InfoLevel
	}
	log.GetLogger(ctx).Logger.SetLevel(l)
}
