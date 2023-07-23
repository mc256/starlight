/*
   file created by Junlin Chen in 2023

*/

package install

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/containerd/containerd/log"
	srvconfig "github.com/containerd/containerd/services/server/config"
	"github.com/mc256/starlight/util"
	toml "github.com/pelletier/go-toml"

	"github.com/urfave/cli/v2"
)

func InstallWithContainerd(ctx context.Context, configPath string) error {
	log.G(ctx).
		WithField("configPath", configPath).
		Info("install Starlight to containerd")
	// check config file exist
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			log.G(ctx).
				WithField("configPath", configPath).
				Warn("config file not exist")

			// generate default config file
			defaultBuffer, err := exec.Command("containerd", "config", "default").Output()
			if err != nil {
				log.G(ctx).
					WithError(err).
					WithField("configPath", configPath).
					Error("generate default config file failed")
				return err
			}

			// create directory for config file
			if err := os.MkdirAll(path.Dir(configPath), 0755); err != nil {
				log.G(ctx).
					WithError(err).
					WithField("configPath", configPath).
					Error("create directory for config file failed")
				return err
			}

			// write default config file
			if err := ioutil.WriteFile(configPath, defaultBuffer, 0644); err != nil {
				log.G(ctx).
					WithError(err).
					WithField("configPath", configPath).
					Error("write default config file failed")
				return err
			}
		} else {
			log.G(ctx).
				WithField("configPath", configPath).
				Error("check config file exist failed, do we need root permission?")
		}
	}

	// add starlight config to containerd config
	reader, err := os.ReadFile(configPath)
	if err != nil {
		log.G(ctx).
			WithError(err).
			WithField("configPath", configPath).
			Error("open config file failed")
		return err
	}

	var config srvconfig.Config
	err = toml.Unmarshal(reader, &config)
	if err != nil {
		log.G(ctx).
			WithError(err).
			WithField("configPath", configPath).
			Error("unmarshal config file failed")
		return err
	}

	// add starlight config
	config.ProxyPlugins["starlight"] = srvconfig.ProxyPlugin{
		Type:    "snapshot",
		Address: "/run/starlight/starlight-snapshotter.sock",
	}

	plugin := config.Plugins["io.containerd.grpc.v1.cri"]
	cni := plugin.Get("cni").(*toml.Tree)
	cni.Set("snapshotter", "starlight")

	// marshal config file
	bytes, err := toml.Marshal(config)
	if err != nil {
		log.G(ctx).
			WithError(err).
			Error("marshal config file failed")
		return err
	}

	// write config file
	if err := ioutil.WriteFile(configPath, bytes, 0644); err != nil {
		log.G(ctx).
			WithError(err).
			WithField("configPath", configPath).
			Error("write config file failed")
		return err
	}

	log.G(ctx).
		WithField("configPath", configPath).
		Info("install Starlight to containerd success")

	return nil
}

func InstallWithK3s(ctx context.Context, configPath string) error {
	// check config file exist

	return nil
}

func Action(ctx context.Context, c *cli.Context) error {
	util.ConfigLoggerWithLevel(c.String("log-level"))

	setK3s := c.Bool("k3s")
	setContainerd := c.Bool("containerd")
	if setK3s && setContainerd {
		return fmt.Errorf("you can only specify excatly one runtime either --k3s or --containerd")
	}
	if !setK3s && !setContainerd {
		return fmt.Errorf("you must specify excatly one runtime either --k3s or --containerd")
	}
	if setContainerd {
		return InstallWithContainerd(ctx, c.String("containerd-config"))
	} else {
		// set k3s
		return InstallWithK3s(ctx, c.String("k3s-config"))
	}

}

func Command() *cli.Command {
	ctx := context.Background()
	return &cli.Command{
		Name:  "install",
		Usage: "install starlight daemon on the host.",
		Action: func(c *cli.Context) error {
			return Action(ctx, c)
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "containerd",
				Value: false,
				Usage: "enable Starlight Daemon for containerd",
			},
			&cli.StringFlag{
				Name:  "containerd-config",
				Value: "/etc/containerd/config.toml",
				Usage: "containerd config file path. If the configuration does not exist, it will be created automatically using the default configuration",
			},
			&cli.BoolFlag{
				Name:  "k3s",
				Value: false,
				Usage: "enable Starlight Daemon for k3s",
			},
			&cli.StringFlag{
				Name:  "k3s-config",
				Value: "/etc/rancher/k3s/config.yaml",
				Usage: "k3s config file path. If the configuration does not exist, it will be created automatically using the default configuration",
			},
		},
		ArgsUsage: "[flags] [BaseImage] PullImage",
	}
}
