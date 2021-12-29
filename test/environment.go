package test

import (
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"
)

func init() {
	if err := godotenv.Load("../.env"); err != nil {
		fmt.Print("Failed to load environment variables from `.env` file. ")
	}
}

func GetContainerRegistry(t *testing.T) (val string) {
	if val = os.Getenv("STARLIGHT_CONTAINER_REGISTRY"); val == "" {
		val = "http://registry:5000"
		t.Fatalf("Environment variable STARLIGHT_CONTAINER_REGISTRY is undefined, using %s", val)
	}
	return val
}

func GetSandboxDirectory(t *testing.T) (val string) {
	if val = os.Getenv("STARLIGHT_SANDBOX_DIR"); val == "" {
		val = "./.sandbox"
		t.Fatalf("Environment variable STARLIGHT_SANDBOX_DIR is undefined, using %s", val)
	}
	return val
}

func GetProxyDBName() string {
	return "proxy_metadata.db"
}