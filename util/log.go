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
	"strings"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/sirupsen/logrus"
)

func ConfigLogger() (ctx context.Context) {
	return ConfigLoggerWithLevel("info")
}

func ConfigLoggerWithLevel(level string) (ctx context.Context) {
	level = strings.ToLower(level)

	// Logger
	ctx = context.Background()
	log.GetLogger(ctx).Logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.StampNano,
		//ForceColors:     true,
		//DisableColors: false,
	})

	switch level {

	case "fatal":
		log.GetLogger(ctx).Logger.SetLevel(logrus.FatalLevel)
		return
	case "error":
		log.GetLogger(ctx).Logger.SetLevel(logrus.ErrorLevel)
		return
	case "warning":
		log.GetLogger(ctx).Logger.SetLevel(logrus.WarnLevel)
		return
	case "info":
		log.GetLogger(ctx).Logger.SetLevel(logrus.InfoLevel)
		return
	case "debug":
		log.GetLogger(ctx).Logger.SetLevel(logrus.DebugLevel)
		return
	case "trace":
		log.GetLogger(ctx).Logger.SetLevel(logrus.TraceLevel)
		return
	default:
		log.GetLogger(ctx).Logger.SetLevel(logrus.InfoLevel)
		return
	}
}
