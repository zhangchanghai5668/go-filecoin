package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gx/ipfs/QmSkuaNgyGmV8c1L3cZNWcUxRJV6J3nsD96JVQPcWcwtyW/go-hamt-ipld"
	"gx/ipfs/QmcD7SqfyQyA91TZUQ7VPRYbGarxmY7EsQewVYMuN5LNSv/go-ipfs-blockstore"
	"gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests given this flag MUST have issues created to address the changes need to fix the tests
var nerfTests = flag.Bool("nerftests", false, "used to nerf tests that block CI") // nolint: deadcode

func TestAddFakeChain(t *testing.T) {
	if !*nerfTests {
		t.SkipNow()
	}
	assert := assert.New(t)
	require := require.New(t)

	var length = 9
	var gbbCount, pbCount int
	ctx := context.Background()

	getHeaviestTipSet := func() core.TipSet {
		gbbCount++
		return core.RequireNewTipSet(require, new(types.Block))
	}
	processBlock := func(context context.Context, block *types.Block) (core.BlockProcessResult, error) {
		pbCount++
		return 0, nil
	}
	loadState := func(context context.Context, ts core.TipSet) (state.Tree, error) {
		return state.NewEmptyStateTree(hamt.NewCborStore()), nil
	}
	fake(ctx, length, false, getHeaviestTipSet, processBlock, loadState)
	assert.Equal(1, gbbCount)
	assert.Equal(length, pbCount)
}

func TestAddActors(t *testing.T) {
	if !*nerfTests {
		t.SkipNow()
	}
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()

	ds := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(ds)
	cm, _ := getChainManager(ds, bs)

	err := cm.Genesis(ctx, core.InitGenesis)
	require.NoError(err)

	st, cst, cm, bts, err := getStateTree(ctx, ds, bs)
	require.NoError(err)

	_, allActors := state.GetAllActors(st)
	initialActors := len(allActors)

	err = fakeActors(ctx, cst, cm, bs, bts)
	assert.NoError(err)

	st, _, _, _, err = getStateTree(ctx, ds, bs)
	require.NoError(err)

	_, allActors = state.GetAllActors(st)
	assert.Equal(initialActors+2, len(allActors), "add a account and miner actors")

	sma, err := st.GetActor(ctx, address.StorageMarketAddress)
	require.NoError(err)

	var storageMkt storagemarket.State
	chunk, err := ds.Get(datastore.NewKey(sma.Head.KeyString()))
	require.NoError(err)
	chunkBytes, ok := chunk.([]byte)
	require.True(ok)
	err = actor.UnmarshalStorage(chunkBytes, &storageMkt)
	require.NoError(err)

	assert.Equal(1, len(storageMkt.Miners))
	assert.Equal(1, len(storageMkt.Orderbook.StorageAsks))
	assert.Equal(1, len(storageMkt.Orderbook.Bids))
}

func GetFakecoinBinary() (string, error) {
	bin := filepath.FromSlash(fmt.Sprintf("%s/src/github.com/filecoin-project/go-filecoin/tools/go-fakecoin/go-fakecoin", os.Getenv("GOPATH")))
	_, err := os.Stat(bin)
	if err == nil {
		return bin, nil
	}

	if os.IsNotExist(err) {
		return "", fmt.Errorf("You are missing the fakecoin binary...try building, searched in '%s'", bin)
	}

	return "", err
}

var testRepoPath = filepath.FromSlash("/tmp/fakecoin/")

func TestCommandsSucceed(t *testing.T) {
	if !*nerfTests {
		t.SkipNow()
	}
	assert := assert.New(t)
	require := require.New(t)

	fbin, err := th.GetFilecoinBinary()
	require.NoError(err)

	os.RemoveAll(testRepoPath)       // go-filecoin init will fail if repo exists.
	defer os.RemoveAll(testRepoPath) // clean up when we're done.

	exec.Command(fbin, "init", "--repodir", testRepoPath).Run()
	require.NoError(err)

	bin, err := GetFakecoinBinary()
	require.NoError(err)

	// 'go-fakecoin fake' completes without error.
	cmdFake := exec.Command(bin, "fake", "-repodir", testRepoPath)
	err = cmdFake.Run()
	assert.NoError(err)

	// 'go-fakecoin actors' completes without error.
	cmdActors := exec.Command(bin, "actors", "-repodir", testRepoPath)
	err = cmdActors.Run()
	assert.NoError(err)
}
