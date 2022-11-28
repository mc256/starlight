/*
   file created by Junlin Chen in 2022

*/

package snapshotter

import (
	"context"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/containerd/snapshots/storage"
	"github.com/mc256/starlight/util"
	"testing"
)

func TestSnapshotList(t *testing.T) {
	socket := "/run/k3s/containerd/containerd.sock"
	ns := "k8s.io"

	client, err := containerd.New(socket, containerd.WithDefaultNamespace(ns))
	if err != nil {
		t.Error(err)
		return
	}
	defer client.Close()

	ctx := context.Background()

	sns := client.SnapshotService("starlight")

	sns.Walk(ctx, func(ctx context.Context, info snapshots.Info) error {
		fmt.Println(info.Name, info.Parent)
		fmt.Println("-->", info.Labels[util.SnapshotLabelRefImage])
		ssid, inf, p, err := storage.GetInfo(ctx, info.Name)
		if err != nil {
			fmt.Println(ssid, inf, p)
		} else {
			fmt.Println(err)
		}
		return nil
	})

	/*
				36c584644174b103238ecd17cf43487b38485bf321f64c8cf525fafc5580ab86 sha256:1021ef88c7974bfff89c5a0ec4fd3160daac6c48a075f74cff721f85dd104e68
				-->
				40b6e9770e3ec864efd193670e72640ed5a9364f2d1287234fa4b8d7731a9cba sha256:68ec11adbdc694363fbd1026d772664049569ee10fc3109b0fc44cc890b60c2c
				-->
				47aac9e355fe94ed399ea2dcbbac7e4c912eb936eb80941d8bf08537f2956c5e sha256:1021ef88c7974bfff89c5a0ec4fd3160daac6c48a075f74cff721f85dd104e68
				-->
				9823e5be2c32d3b2547b9a5344d49628e38189a4beafbb6c7dee3858545b98f1 sha256:ef04ecfb1d007266224a5b5b67992c74afdf12e984433a6978d061c7b852ee10
				-->
				c7f1c4db6e6fdb52efd09abeb5e51e7af6dc6fc753e32c34e4f9f04544dcd6cc sha256:ef04ecfb1d007266224a5b5b67992c74afdf12e984433a6978d061c7b852ee10
				-->
				sha256:1021ef88c7974bfff89c5a0ec4fd3160daac6c48a075f74cff721f85dd104e68
				-->
				sha256:1ad27bdd166b922492031b1938a4fb2f775e3d98c8f1b72051dad0570a4dd1b5
				-->
				sha256:40cf597a9181e86497f4121c604f9f0ab208950a98ca21db883f26b0a548a2eb
				-->
				sha256:4ac94bd63114d70c68e73d408d559bbd681b7c5094715b1a38bd7b361312555f sha256:1ad27bdd166b922492031b1938a4fb2f775e3d98c8f1b72051dad0570a4dd1b5
				-->
				sha256:68ec11adbdc694363fbd1026d772664049569ee10fc3109b0fc44cc890b60c2c sha256:7ad6b790083aeda0ad3e5e2888d9cf1a64d6c5ff6b41487a3404bc6f5684a2d3
				--> sha256:c20f5ef500bc93f26fec480dfea1d4cbda2d39791d611031a3aaf3c096f35c73
				sha256:7ad6b790083aeda0ad3e5e2888d9cf1a64d6c5ff6b41487a3404bc6f5684a2d3 sha256:b11040115c1a7bad75c256bfad158a1bf2aec69418ffc1f49326ba59e33f69b2
				--> sha256:c20f5ef500bc93f26fec480dfea1d4cbda2d39791d611031a3aaf3c096f35c73
				sha256:a5878da390fcf0e6324ce5593f03ae9609caaa9bce0a522b504c00b91e579b46
				--> sha256:c20f5ef500bc93f26fec480dfea1d4cbda2d39791d611031a3aaf3c096f35c73
				sha256:b11040115c1a7bad75c256bfad158a1bf2aec69418ffc1f49326ba59e33f69b2 sha256:d44df6880f5a4e309d25a92dc413458a1f964493868c99000519dbc365abe1d7
				--> sha256:c20f5ef500bc93f26fec480dfea1d4cbda2d39791d611031a3aaf3c096f35c73
				sha256:d44df6880f5a4e309d25a92dc413458a1f964493868c99000519dbc365abe1d7 sha256:f8521e5ffbea025f78787bf0d407a4295dff5fcc3cc5c8936126fd17fa1819a8
				--> sha256:c20f5ef500bc93f26fec480dfea1d4cbda2d39791d611031a3aaf3c096f35c73
				sha256:ef04ecfb1d007266224a5b5b67992c74afdf12e984433a6978d061c7b852ee10 sha256:4ac94bd63114d70c68e73d408d559bbd681b7c5094715b1a38bd7b361312555f
				-->
				sha256:f8521e5ffbea025f78787bf0d407a4295dff5fcc3cc5c8936126fd17fa1819a8 sha256:a5878da390fcf0e6324ce5593f03ae9609caaa9bce0a522b504c00b91e579b46
				--> sha256:c20f5ef500bc93f26fec480dfea1d4cbda2d39791d611031a3aaf3c096f35c73


		sha256:c20f5ef500bc93f26fec480dfea1d4cbda2d39791d611031a3aaf3c096f35c73
		manifest

		complete.starlight.mc256.dev=2022-11-27T18:34:46Z,
		puller.containerd.io=starlight,
		mediaType.starlight.mc256.dev=manifest,
		containerd.io/gc.ref.snapshot.starlight/0=sha256:a5878da390fcf0e6324ce5593f03ae9609caaa9bce0a522b504c00b91e579b46,
		containerd.io/gc.ref.content.starlight=sha256:5c409db91624a306224d9a98efcfded2ae94da0a483cf2f1d40b8cb1ea776385,
		starlight.mc256.dev/distribution.source.in-cluster=starlight-registry.default.svc.cluster.local:5000/starlight/redis:6.2.1,
		containerd.io/gc.ref.content.config=sha256:982983f3c76aeb79bddb66239c878f011c147b15cac9d7c71abedf76e772c7c6,
		containerd.io/gc.ref.snapshot.starlight/5=sha256:68ec11adbdc694363fbd1026d772664049569ee10fc3109b0fc44cc890b60c2c,
		containerd.io/gc.ref.snapshot.starlight/4=sha256:7ad6b790083aeda0ad3e5e2888d9cf1a64d6c5ff6b41487a3404bc6f5684a2d3,
		containerd.io/gc.ref.snapshot.starlight/2=sha256:d44df6880f5a4e309d25a92dc413458a1f964493868c99000519dbc365abe1d7,
		containerd.io/gc.ref.snapshot.starlight/1=sha256:f8521e5ffbea025f78787bf0d407a4295dff5fcc3cc5c8936126fd17fa1819a8,
		containerd.io/gc.ref.snapshot.starlight/3=sha256:b11040115c1a7bad75c256bfad158a1bf2aec69418ffc1f49326ba59e33f69b2

		sha256:e813af18bfe9565a5a4b67e4e80be064c488c4630770951c97f240fa337192e8 945B    53 minutes
		containerd.io/gc.ref.content.config=sha256:4b167e69a056cad9f179b344d7208fefcbc99fee9d48c8988098d4af45cbc9ed,
		containerd.io/distribution.source.ghcr.io=mc256/starlight/cli,
		containerd.io/gc.ref.content.l.2=sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1,
		containerd.io/gc.ref.content.l.1=sha256:76fc34d44084250f3a5e66218596e2105e9e0537b69456999dccf1740be1795e,
		containerd.io/gc.ref.content.l.0=sha256:1b7ca6aea1ddfe716f3694edb811ab35114db9e93f3ce38d7dab6b4d9270cb0c

	*/
}
