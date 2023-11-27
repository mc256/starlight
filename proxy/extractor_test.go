/*
   file created by Junlin Chen in 2022

*/

package proxy

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/test"
)

func TestNewExtractor(t *testing.T) {
	ctx := context.Background()
	cfg, _, _, _ := LoadConfig("")
	server := &Server{
		ctx: ctx,
		Server: http.Server{
			Addr: fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort),
		},
		config: cfg,
	}

	ext, err := NewExtractor(server, "starlight/mariadb:10.9.2", true)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(ext)
}

func TestExtractor_SaveToC_Goharbor(t *testing.T) {
	test.LoadEnvironmentVariables()
	if test.HasLoginStarlightGoharbor() == false {
		t.Skip(">>>>> Skip: no container registry credentials for goharbor")
	}

	ctx := context.Background()
	cfg, _, _, _ := LoadConfig("")

	if os.Getenv("TEST_HARBOR_REGISTRY") != "" {
		cfg.DefaultRegistry = os.Getenv("TEST_HARBOR_REGISTRY")
	}
	if os.Getenv("POSTGRES_CONNECTION_URL") != "" {
		cfg.PostgresConnectionString = os.Getenv("POSTGRES_CONNECTION_URL")
	}

	server := &Server{
		ctx: ctx,
		Server: http.Server{
			Addr: fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort),
		},
		config: cfg,
	}
	if db, err := NewDatabase(ctx, cfg.PostgresConnectionString); err != nil {
		log.G(ctx).Errorf("failed to connect to database: %v\n", err)
	} else {
		server.db = db
	}

	ext, err := NewExtractor(server, "starlight/mariadb:10.9.2", true)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(ext)
	res, err := ext.SaveToC()
	if err != nil {
		t.Error(err)
	}
	fmt.Println(res)
}

func TestExtractor_SaveToC_AWSECR(t *testing.T) {
	test.LoadEnvironmentVariables()
	if test.HasLoginAWSECR() == false {
		t.Skip(">>>>> Skip: no container registry credentials for AWS ECR")
	}

	ctx := context.Background()
	cfg, _, _, _ := LoadConfig("")

	if os.Getenv("TEST_HARBOR_REGISTRY") != "" {
		cfg.DefaultRegistry = os.Getenv("TEST_HARBOR_REGISTRY")
	}
	if os.Getenv("POSTGRES_CONNECTION_URL") != "" {
		cfg.PostgresConnectionString = os.Getenv("POSTGRES_CONNECTION_URL")
	}

	if os.Getenv("TEST_ECR_IMAGE_TO") == "" {
		t.Skip(">>>>> Skip: no ECR image set in TEST_ECR_IMAGE_TO")
	}

	awsecrImage := os.Getenv("TEST_ECR_IMAGE_TO")

	server := &Server{
		ctx: ctx,
		Server: http.Server{
			Addr: fmt.Sprintf("%s:%d", cfg.ListenAddress, cfg.ListenPort),
		},
		config: cfg,
	}
	if db, err := NewDatabase(ctx, cfg.PostgresConnectionString); err != nil {
		log.G(ctx).Errorf("failed to connect to database: %v\n", err)
	} else {
		server.db = db
	}

	ext, err := NewExtractor(server, awsecrImage, true)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(ext)
	res, err := ext.SaveToC()
	if err != nil {
		t.Error(err)
	}
	fmt.Println(res)
}
