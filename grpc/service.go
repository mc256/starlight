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

package grpc

import (
	"context"
	snapshotsapi "github.com/containerd/containerd/api/services/snapshots/v1"
	"github.com/containerd/containerd/contrib/snapshotservice"
	"github.com/containerd/containerd/log"
	"google.golang.org/grpc"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func NewSnapshotterGrpcService(ctx context.Context, socketAddress, remoteProtocol, remoteAddress, workDir string, fsTrace bool) {
	// Create a gRPC server
	rpc := grpc.NewServer()

	// Configure your custom snapshotter, this example uses the native
	// snapshotter and a root directory. Your custom snapshotter will be
	// much more useful than using a snapshotter which is already included.
	// https://godoc.org/github.com/containerd/containerd/snapshots#Snapshotter
	remote := NewStarlightProxy(ctx, remoteProtocol, remoteAddress)

	sn, err := NewSnapshotter(ctx, workDir, remote, fsTrace)
	if err != nil {
		log.G(ctx).WithError(err).Fatal("failed to create new snapshotter")
		os.Exit(1)
	}

	// Convert the snapshotter to a gRPC service,
	// example in github.com/containerd/containerd/contrib/snapshotservice
	service := snapshotservice.FromSnapshotter(sn)

	// Prepare the directory for the socket
	if err := os.MkdirAll(filepath.Dir(socketAddress), 0700); err != nil {
		log.G(ctx).WithError(err).Fatalf("failed to create directory %q\n", filepath.Dir(socketAddress))
		os.Exit(1)
	}

	// Try to remove the socket file to avoid EADDRINUSE
	if err := os.RemoveAll(socketAddress); err != nil {
		log.G(ctx).WithError(err).Fatalf("failed to remove %q\n", socketAddress)
		os.Exit(1)
	}
	log.G(ctx).Info("Snapshotter is ready")

	// Register the service with the gRPC server
	snapshotsapi.RegisterSnapshotsServer(rpc, service)

	// Listen and serve
	l, err := net.Listen("unix", socketAddress)
	if err != nil {
		log.G(ctx).WithError(err).Fatal("unix listen")
		os.Exit(1)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	go func() {
		<-c
		rpc.Stop()
	}()

	if err := rpc.Serve(l); err != nil {
		log.G(ctx).WithError(err).Fatal("rpc serve")
		os.Exit(1)
	}

}
