package client

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/name"
	pb "github.com/mc256/starlight/client/api"
	"github.com/mc256/starlight/client/fs"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
)

// -----------------------------------------------------------------------------
// CLI gRPC server
// -----------------------------------------------------------------------------
// StarlightDaemonAPIServer is the gRPC server for the ctr-starlight CLI tool
// Do you want to update the interface, try to run `make update-grpc` in the
// root directory of the project

type StarlightDaemonAPIServer struct {
	pb.UnimplementedDaemonServer
	client *Client
}

func (s *StarlightDaemonAPIServer) GetVersion(ctx context.Context, req *pb.Request) (*pb.Version, error) {
	return &pb.Version{
		Version: util.Version,
	}, nil
}

func (s *StarlightDaemonAPIServer) AddProxyProfile(ctx context.Context, req *pb.AuthRequest) (*pb.AuthResponse, error) {
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"protocol": req.Protocol,
		"address":  req.Address,
		"username": req.Username,
	}).Debug("grpc: add proxy profile")

	s.client.cfg.Proxies[req.ProfileName] = &ProxyConfig{
		Protocol: req.Protocol,
		Address:  req.Address,
		Username: req.Username,
		Password: req.Password,
	}
	if err := s.client.cfg.SaveConfig(); err != nil {
		log.G(s.client.ctx).WithError(err).Errorf("failed to save config")
		return &pb.AuthResponse{
			Success: false,
			Message: "failed to save config",
		}, nil
	}
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"protocol": req.Protocol,
		"address":  req.Address,
		"username": req.Username,
	}).Info("add auth profile")
	return &pb.AuthResponse{
		Success: true,
	}, nil
}

func (s *StarlightDaemonAPIServer) GetProxyProfiles(ctx context.Context, req *pb.Request) (resp *pb.GetProxyProfilesResponse, err error) {
	log.G(s.client.ctx).Debug("grpc: get proxy profiles")

	profiles := []*pb.GetProxyProfilesResponse_Profile{}
	for name, proxy := range s.client.cfg.Proxies {
		profiles = append(profiles, &pb.GetProxyProfilesResponse_Profile{
			Name:     name,
			Protocol: proxy.Protocol,
			Address:  proxy.Address,
		})
	}
	return &pb.GetProxyProfilesResponse{
		Profiles: profiles,
	}, nil
}

func (s *StarlightDaemonAPIServer) PullImage(ctx context.Context, ref *pb.ImageReference) (resp *pb.ImagePullResponse, err error) {
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"base":   ref.Base,
		"ref":    ref.Reference,
		"socket": s.client.cfg.Containerd,
	}).Debug("grpc: pull image")

	ns := ref.Namespace
	if ns == "" {
		ns = s.client.cfg.Namespace
	}

	ready := make(chan PullFinishedMessage)

	go s.client.pullImageGrpc(ns, ref.Base, ref.Reference, ref.ProxyConfig, &ready, ref.DisableEarlyStart)
	ret := <-ready

	if ret.err != nil {
		if ret.img != nil {
			// requested image is already pulled
			log.G(s.client.ctx).WithFields(logrus.Fields{
				"_base":   ret.base,
				"_ref":    ref.Reference,
				"message": ret.err.Error(),
			}).Warn("image already pulled")
			return &pb.ImagePullResponse{
				Success:        true,
				Message:        ret.err.Error(),
				BaseImage:      ret.base,
				TotalImageSize: -1,
			}, nil
		} else {
			log.G(s.client.ctx).WithFields(logrus.Fields{
				"_base":   ret.base,
				"_ref":    ref.Reference,
				"message": ret.err.Error(),
			}).Error("failed to pull image")
			return &pb.ImagePullResponse{
				Success:        false,
				Message:        ret.err.Error(),
				BaseImage:      ret.base,
				TotalImageSize: -1,
			}, nil
		}
	}

	// wait for the entire delta image to be pulled to the local filesystem
	// holding on the second signal from the pullImageGrpc goroutine
	if ref.DisableEarlyStart {
		ret := <-ready // second signal
		if ret.err != nil {
			log.G(s.client.ctx).WithFields(logrus.Fields{
				"_base":   ret.base,
				"_ref":    ref.Reference,
				"message": ret.err.Error(),
			}).Error("failed to pull image")
			return &pb.ImagePullResponse{
				Success:        false,
				Message:        ret.err.Error(),
				BaseImage:      ret.base,
				TotalImageSize: -1,
			}, nil
		}
	}

	// success
	return &pb.ImagePullResponse{
		Success:           true,
		Message:           "ok",
		BaseImage:         ret.base,
		TotalImageSize:    ret.meta.ContentLength,
		OriginalImageSize: ret.meta.OriginalLength,
	}, nil
}

func (s *StarlightDaemonAPIServer) SetOptimizer(ctx context.Context, req *pb.OptimizeRequest) (*pb.OptimizeResponse, error) {
	okRes, failRes := make(map[string]string), make(map[string]string)
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"enable": req.Enable,
	}).Debug("grpc: set optimizer")

	s.client.optimizerLock.Lock()
	defer s.client.optimizerLock.Unlock()

	s.client.managerMapLock.Lock()
	defer s.client.managerMapLock.Unlock()

	if req.Enable {

		s.client.defaultOptimizer = true
		s.client.defaultOptimizeGroup = req.Group

		for d, m := range s.client.managerMap {
			if st, err := m.SetOptimizerOn(req.Group); err != nil {
				log.G(s.client.ctx).
					WithField("group", req.Group).
					WithField("md", d).
					WithField("start", st).
					WithError(err).
					Error("failed to set optimizer on")
				failRes[d] = err.Error()
			} else {
				okRes[d] = time.Now().Format(time.RFC3339)
			}
		}

	} else {

		s.client.defaultOptimizer = false
		s.client.defaultOptimizeGroup = ""

		for d, m := range s.client.managerMap {
			if et, err := m.SetOptimizerOff(); err != nil {
				log.G(s.client.ctx).
					WithField("md", d).
					WithField("duration", et).
					WithError(err).
					Error("failed to set optimizer off")
				failRes[d] = err.Error()
			} else {
				okRes[d] = fmt.Sprintf("collected %.3fs file access traces", et.Seconds())
			}
		}
	}

	return &pb.OptimizeResponse{
		Success: true,
		Message: "completed request",
		Okay:    okRes,
		Failed:  failRes,
	}, nil
}

func (s *StarlightDaemonAPIServer) ReportTraces(ctx context.Context, req *pb.ReportTracesRequest) (*pb.ReportTracesResponse, error) {
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"profile": req.ProxyConfig,
	}).Debug("grpc: report")

	tc, err := fs.NewTraceCollection(s.client.ctx, s.client.cfg.TracesDir)
	if err != nil {
		return &pb.ReportTracesResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	err = s.client.UploadTraces(req.ProxyConfig, tc)
	if err != nil {
		return &pb.ReportTracesResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.ReportTracesResponse{
		Success: true,
		Message: "uploaded traces",
	}, nil
}

func (s *StarlightDaemonAPIServer) NotifyProxy(ctx context.Context, req *pb.NotifyRequest) (*pb.NotifyResponse, error) {
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"profile": req.ProxyConfig,
	}).Debug("grpc: notify")

	reference, err := name.ParseReference(req.Reference)
	if err != nil {
		return &pb.NotifyResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	err = s.client.Notify(req.ProxyConfig, reference, true)
	if err != nil {
		return &pb.NotifyResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.NotifyResponse{
		Success: true,
		Message: reference.String(),
	}, nil
}

func (s *StarlightDaemonAPIServer) PingTest(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	log.G(s.client.ctx).WithFields(logrus.Fields{
		"profile": req.ProxyConfig,
	}).Debug("grpc: ping test")

	rtt, proto, server, err := s.client.Ping(req.ProxyConfig)
	if err != nil {
		return &pb.PingResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.PingResponse{
		Success: true,
		Message: fmt.Sprintf("ok! - %s://%s", proto, server),
		Latency: rtt,
	}, nil
}

func newStarlightDaemonAPIServer(client *Client) *StarlightDaemonAPIServer {
	c := &StarlightDaemonAPIServer{client: client}
	return c
}
