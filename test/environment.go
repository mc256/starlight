package test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// IsDevEnvironment returns true if the STARLIGHT_ENV environment variable starts with "DEV".
// export STARLIGHT_ENV=dev
func IsDevEnvironment() bool {
	return strings.HasPrefix(strings.ToUpper(os.Getenv("STARLIGHT_ENV")), "DEV")
}

func checkDockerConfig(domainName string) bool {
	type dockerConfig struct {
		Auths map[string]struct{}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	buf, err := os.ReadFile(fmt.Sprintf("%s/.docker/config.json", home))
	if err != nil {
		return false
	}

	cfg := &dockerConfig{}
	err = json.Unmarshal(buf, cfg)
	if err != nil {
		return false
	}

	_, has := cfg.Auths[domainName]
	return has
}

func HasLoginAWSECR() bool {
	// aws ecr-public get-login-password --region us-east-1 --profile $AWS_PROFILE | docker login --username AWS --password-stdin $DOMAIN
	return checkDockerConfig("public.ecr.aws")
}

func HasLoginStarlightGoharbor() bool {
	return checkDockerConfig("harbor.starlight.mc256.dev")
}
