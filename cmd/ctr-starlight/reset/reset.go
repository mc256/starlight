package reset

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/util"
	"github.com/urfave/cli/v2"
)

func terminateContainerd(ctx context.Context) error {
	log.G(ctx).
		Info("terminating containerd")

	out, _ := exec.Command("systemctl", "stop", "containerd").CombinedOutput()
	if outs := string(out); strings.TrimSpace(outs) != "" {
		log.G(ctx).
			Warnf("systemctl stop containerd: %s", outs)
	}

	if err := os.RemoveAll("/var/lib/containerd"); err != nil {
		log.G(ctx).
			WithError(err).
			Warnf("rm -rf /var/lib/containerd")
	}
	if err := os.RemoveAll("/run/containerd"); err != nil {
		log.G(ctx).
			WithError(err).
			Warnf("rm -rf /run/containerd")
	}

	out, _ = exec.Command("pkill", "-9", "containerd").CombinedOutput()
	if outs := string(out); strings.TrimSpace(outs) != "" {
		log.G(ctx).
			Warnf("pkill -9 containerd: %s", outs)
	}

	return nil
}

func unmountFs(ctx context.Context) error {
	log.G(ctx).
		Info("unmounting filesystems")

	// iterate all files in /var/lib/starlight
	files, err := ioutil.ReadDir("/var/lib/starlight/sfs")
	if err != nil {
		return nil
	}

	for _, f := range files {
		// if it is a directory
		if f.IsDir() {
			// if it is a mount point
			log.G(ctx).
				WithField("path", path.Join(f.Name(), "m")).
				Info("unmounting")

			if s, err := os.Stat(path.Join(f.Name(), "m")); err == nil && s.IsDir() {
				out, err := exec.Command("umount", path.Join(f.Name(), "m")).CombinedOutput()
				if outs := string(out); strings.TrimSpace(outs) != "" {
					log.G(ctx).
						WithField("path", path.Join(f.Name(), "m")).
						Warnf("umount: %s", outs)
				}
				if err != nil {
					out, _ := exec.Command("umount", "-l", path.Join(f.Name(), "m")).CombinedOutput()
					if outs := string(out); strings.TrimSpace(outs) != "" {
						log.G(ctx).
							WithField("path", path.Join(f.Name(), "m")).
							Warnf("umount -l: %s", outs)
					}
				}
			}
		}
	}

	return nil
}

func terminateStarlight(ctx context.Context) error {
	log.G(ctx).
		Info("terminating starlight")

	out, _ := exec.Command("systemctl", "stop", "starlight").CombinedOutput()
	if outs := string(out); strings.TrimSpace(outs) != "" {
		log.G(ctx).
			Warnf("systemctl stop starlight: %s", outs)
	}

	if err := os.RemoveAll("/var/lib/starlight"); err != nil {
		log.G(ctx).
			WithError(err).
			Warnf("rm -rf /var/lib/starlight")
	}
	if err := os.RemoveAll("/run/starlight"); err != nil {
		log.G(ctx).
			WithError(err).
			Warnf("rm -rf /run/starlight")
	}

	out, _ = exec.Command("pkill", "-9", "starlight-d").CombinedOutput()
	if outs := string(out); strings.TrimSpace(outs) != "" {
		log.G(ctx).
			Warnf("pkill -9 starlight-d: %s", outs)
	}

	return nil
}

func Action(ctx context.Context, c *cli.Context) error {
	util.ConfigLoggerWithLevel(c.String("log-level"))

	all := c.Bool("all")
	containerd := c.Bool("containerd")
	starlight := c.Bool("starlight")
	umount := c.Bool("umount")

	if all {
		containerd = true
		starlight = true
		umount = true
	}

	if containerd {
		_ = terminateContainerd(ctx)
	}

	if umount {
		_ = unmountFs(ctx)
	}

	if starlight {
		_ = terminateStarlight(ctx)
	}

	return nil
}

func Command() *cli.Command {
	ctx := context.Background()
	return &cli.Command{
		Name:  "reset",
		Usage: "reset all services for starlight daemon on the host (for benchmark or debug purpose)",
		Action: func(c *cli.Context) error {
			return Action(ctx, c)
		},
		Flags: []cli.Flag{
			// all
			&cli.BoolFlag{
				Name:  "all",
				Value: false,
				Usage: "reset all the known services for starlight daemon on the host",
			},

			// 3rd party
			&cli.BoolFlag{
				Name:  "containerd",
				Value: false,
				Usage: "enable Starlight Daemon for containerd",
			},

			// starlight
			&cli.BoolFlag{
				Name:  "starlight",
				Value: false,
				Usage: "reset starlight daemon",
			},
			&cli.BoolFlag{
				Name:  "umount",
				Value: false,
				Usage: "unmount starlight rootfs",
			},
		},
		ArgsUsage: "",
	}
}
