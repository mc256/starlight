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
	"github.com/mc256/starlight/client"
	"github.com/mc256/starlight/util"
	toml "github.com/pelletier/go-toml"

	"github.com/urfave/cli/v2"
)

func appendStarlightConfiguartion(ctx context.Context, socketAddress string, reader []byte) (output []byte, err error) {
	var config srvconfig.Config
	err = toml.Unmarshal(reader, &config)
	if err != nil {
		return nil, err
	}

	// add starlight config
	if config.ProxyPlugins == nil {
		config.ProxyPlugins = make(map[string]srvconfig.ProxyPlugin)
	}

	config.ProxyPlugins["starlight"] = srvconfig.ProxyPlugin{
		Type:    "snapshot",
		Address: socketAddress,
	}

	plugin := config.Plugins["io.containerd.grpc.v1.cri"]
	cni := plugin.Get("cni").(*toml.Tree)
	cni.Set("snapshotter", "starlight")

	// marshal config file
	output, err = toml.Marshal(config)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func InstallWithContainerd(ctx context.Context, configPath, socketAddr string) error {
	log.G(ctx).
		WithField("configPath", configPath).
		Info("installing Starlight to containerd")

	// check config file exist
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			log.G(ctx).
				WithField("configPath", configPath).
				Warn("config file not exist")

			// generate default config file
			defaultBuffer, err := exec.Command("containerd", "config", "default").Output()
			if err != nil {
				return fmt.Errorf("generate default config file failed: %w", err)
			}

			// create directory for config file
			if err := os.MkdirAll(path.Dir(configPath), 0755); err != nil {
				return fmt.Errorf("create directory for config file failed: %w", err)
			}

			// write default config file
			if err := ioutil.WriteFile(configPath, defaultBuffer, 0644); err != nil {
				return fmt.Errorf("write default config file failed: %w", err)
			}
		} else {
			return fmt.Errorf("check config file exist failed (do we need root permission?): %w", err)
		}
	}

	// add starlight config to containerd config
	reader, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("open config file failed: %w", err)
	}

	// append starlight config
	bytes, err := appendStarlightConfiguartion(ctx, socketAddr, reader)
	if err != nil {
		return fmt.Errorf("append starlight config to containerd config failed: %w", err)
	}

	// write config file
	if err := ioutil.WriteFile(configPath, bytes, 0644); err != nil {
		return fmt.Errorf("write config file failed: %w", err)
	}

	log.G(ctx).
		Info("successfully install Starlight to containerd")

	return nil
}

func InstallWithK3s(ctx context.Context, configPath, configTemplatePath, socketAddr, config string) error {
	log.G(ctx).
		WithField("configPath", configPath).
		WithField("configTemplatePath", configTemplatePath).
		Info("installing Starlight to containerd")

	// check config files exist
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("cannot find k3s config file, do we need root permission? or have you install k3s?: %w", err)
		} else {
			return fmt.Errorf("check config file exist failed, do we need root permission?: %w", err)
		}
	}

	// read config file
	reader, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("open config file failed: %w", err)
	}

	// append starlight config
	bytes, err := appendStarlightConfiguartion(ctx, socketAddr, reader)
	if err != nil {
		return fmt.Errorf("append starlight config to k3s config failed: %w", err)
	}

	// write config template file
	if err := ioutil.WriteFile(configTemplatePath, bytes, 0644); err != nil {
		return fmt.Errorf("write config template file failed: %w", err)
	}

	// update starlight config
	cfg, _, _, _ := client.LoadConfig(config)
	cfg.Containerd = "/run/k3s/containerd/containerd.sock"
	if err = cfg.SaveConfig(); err != nil {
		return fmt.Errorf("update starlight config failed: %w", err)
	}

	log.G(ctx).
		Info("successfully install Starlight to k3s")

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
		// set containerd
		return InstallWithContainerd(ctx, c.String("containerd-config"), c.String("starlight-socket"))
	} else {
		// set k3s
		return InstallWithK3s(ctx,
			c.String("k3s-config"), c.String("k3s-config-template"),
			c.String("starlight-socket"), c.String("starlight-config"),
		)
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
				Usage: "enable Starlight Daemon for k3s and annotate the node with \"starlight./enable=true\"",
			},
			&cli.StringFlag{
				Name:  "k3s-config",
				Value: "/var/lib/rancher/k3s/agent/etc/containerd/config.toml",
				Usage: "k3s config file path. This file is generated by k3s and cannot be modified.",
			},
			&cli.StringFlag{
				Name:  "k3s-config-template",
				Value: "/var/lib/rancher/k3s/agent/etc/containerd/config.toml.tmpl",
				Usage: "k3s config template file path. If the template does not exist, it will be created automatically using the existing configration",
			},
			&cli.StringFlag{
				Name:  "starlight-socket",
				Value: "/run/starlight/starlight-snapshotter.sock",
				Usage: "starlight snapshotter socket path for configuration",
			},
			&cli.StringFlag{
				Name:  "starlight-config",
				Value: "/etc/starlight/starlight-daemon.json",
				Usage: "starlight config file path. If the configuration does not exist, it will be created automatically using the default configuration",
			},
		},
		ArgsUsage: "(--containerd|--k3s)",
	}
}
