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

package ctr

import (
	"context"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/mc256/starlight/util"
)

type ContainerdClient struct {
	namespaces string
	ctx        context.Context

	Container  containerd.Container
	Task       containerd.Task
	ExitSignal <-chan containerd.ExitStatus

	Client *containerd.Client
	Sn     *SnapshotterService
	SnId   string
}

func NewContainerdClient(namespace, socket, logLevel string) (c *ContainerdClient, ctx context.Context, err error) {
	var client *containerd.Client
	if client, err = containerd.New(socket); err != nil {
		return nil, nil, err
	}

	c = &ContainerdClient{
		namespaces: namespace,
		Client:     client,
	}

	util.ConfigLoggerWithLevel(logLevel)
	c.ctx = namespaces.WithNamespace(context.Background(), namespace)

	// Snapshotter service
	c.Sn = NewSnapshotterService(c.ctx, c.Client)
	return c, c.ctx, nil
}
