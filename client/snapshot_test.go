/*
   file created by Junlin Chen in 2022

*/

package client

import (
	"context"
	"fmt"
	"github.com/containerd/containerd/snapshots"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	"testing"
	"time"
)

func Test_diffIds(t *testing.T) {
	diffs := []digest.Digest{"sha256:bad4c2b21ac784690edb9fab9336c06f429523122db8258386b0816ea9ccff13",
		"sha256:541172e54f7a947125460b76b5788026aa93b9cfb6a9ee132159bd792fc5a213",
		"sha256:a0daea3da3a891e0907836e15387b107376b938918839afd555a93f56a04a437",
		"sha256:c21bfa4fc8f93fcf8cee7f410285c9b0830c39e581d122668f1bfac657d01539",
		"sha256:2f02d7c1705eec01163defce0c73bad60ef4696b9fd2e009bf64f15425e3cb9b",
		"sha256:56b9282b4ec2b115a530fb08958abc2dee400cebc1befbdc3d2a70ee8a7afc97"}
	chainId := identity.ChainID(diffs)
	fmt.Println(chainId)
	// sha256:b2defa2545685fad9251740b472ecc07e0334f652a973ec0f37bf55ba9917b70
}

func Test_diffIds2(t *testing.T) {
	diffs := []digest.Digest{"sha256:0ee7044be87efdc00d5c40caa3193d2192eb09e8d136cb9cc2ab9aa82864b6c1",
		"sha256:0ee7044be87efdc00d5c40caa3193d2192eb09e8d136cb9cc2ab9aa82864b6c1"}
	chainId := identity.ChainID(diffs)
	fmt.Println(chainId)
}

func Test_SnapshotCommit(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	ctx := context.TODO()
	sn, err := NewSnapshotter(ctx, cfg)
	if err != nil {
		t.Error(err)
		return
	}
	tempId := "alpha-test"
	chainId := "sha256:bad4c2b21ac784690edb9fab9336c06f429523122db8258386b0816ea9ccff13"
	err = sn.addSnapshot(tempId, "")
	if err != nil {
		t.Error(err)
		return
	}

	err = sn.commitSnapshot(chainId, tempId)
	if err != nil {
		t.Error(err)
		return
	}

	info, err := sn.Stat(ctx, chainId)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println(info)

	/*
		err = sn.removeSnapshot(chainId)
		if err != nil {
			t.Error(err)
			return
		}
	*/
}

func Test_SnapshotCommit2(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	ctx := context.TODO()
	sn, err := NewSnapshotter(ctx, cfg)
	if err != nil {
		t.Error(err)
		return
	}

	diffs := []digest.Digest{"sha256:bad4c2b21ac784690edb9fab9336c06f429523122db8258386b0816ea9ccff13",
		"sha256:541172e54f7a947125460b76b5788026aa93b9cfb6a9ee132159bd792fc5a213",
		"sha256:a0daea3da3a891e0907836e15387b107376b938918839afd555a93f56a04a437",
		"sha256:c21bfa4fc8f93fcf8cee7f410285c9b0830c39e581d122668f1bfac657d01539",
		"sha256:2f02d7c1705eec01163defce0c73bad60ef4696b9fd2e009bf64f15425e3cb9b",
		"sha256:56b9282b4ec2b115a530fb08958abc2dee400cebc1befbdc3d2a70ee8a7afc97"}

	chainIds := identity.ChainIDs(diffs)
	prev := ""
	for _, id := range chainIds {
		tempId := getRandomId()
		err = sn.addSnapshot(tempId, prev)
		if err != nil {
			t.Error(err)
			return
		}
		prev = id.String()

		err = sn.commitSnapshot(id.String(), tempId)
		if err != nil {
			t.Error(err)
			return
		}

		info, err := sn.Stat(ctx, id.String())
		if err != nil {
			t.Error(err)
			return
		}
		fmt.Println(info)
	}

	/*
		err = sn.removeSnapshot(chainId)
		if err != nil {
			t.Error(err)
			return
		}
	*/
}

func Test_SnapshotWalk(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	ctx := context.TODO()
	sn, err := NewSnapshotter(ctx, cfg)
	if err != nil {
		t.Error(err)
		return
	}

	err = sn.Walk(ctx, func(ctx context.Context, info snapshots.Info) error {
		fmt.Println(info)

		return nil
	})
	if err != nil {
		t.Error(err)
		return
	}
}

func TestSnapshotter_Remove(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	ctx := context.TODO()
	sn, err := NewSnapshotter(ctx, cfg)
	if err != nil {
		t.Error(err)
		return
	}

	chainId := "sha256:9fb2c14fbbfb2675eb79fc48c2972d2a41b7456d15876b6046c0105cb2104beb"
	err = sn.removeSnapshot(chainId)
	if err != nil {
		t.Error(err)
		return
	}
}

func Test_SnapshotWalk2(t *testing.T) {
	cfg, _, _, _ := LoadConfig("/root/daemon.json")
	ctx := context.TODO()
	sn, err := NewSnapshotter(ctx, cfg)
	if err != nil {
		t.Error(err)
		return
	}

	err = sn.Walk(ctx, func(ctx context.Context, info snapshots.Info) error {
		fmt.Println(info)
		info.Labels = map[string]string{"starlight.mc256.dev/completed": time.Now().Format(time.RFC3339)}
		sn.Update(ctx, info, "labels.starlight.mc256.dev/completed")
		return nil
	})
	if err != nil {
		t.Error(err)
		return
	}
}
