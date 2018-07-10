package commands

import (
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"

	"github.com/stretchr/testify/assert"
)

func TestAddrsNewAndList(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	addrs := make([]string, 10)
	for i := 0; i < 10; i++ {
		addrs[i] = d.CreateWalletAddr()
	}

	list := d.RunSuccess("wallet", "addrs", "ls").ReadStdout()
	for _, addr := range addrs {
		assert.Contains(list, addr)
	}
}

func TestWalletBalance(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()
	addr := d.CreateWalletAddr()

	t.Log("[success] not found, zero")
	balance := d.RunSuccess("wallet", "balance", addr)
	assert.Equal("0", balance.readStdoutTrimNewlines())

	t.Log("[success] balance 10000000")
	balance = d.RunSuccess("wallet", "balance", address.NetworkAddress.String())
	assert.Equal("10000000", balance.readStdoutTrimNewlines())

	t.Log("[success] newly generated one")
	addrNew := d.RunSuccess("wallet addrs new")
	balance = d.RunSuccess("wallet", "balance", addrNew.readStdoutTrimNewlines())
	assert.Equal("0", balance.readStdoutTrimNewlines())
}

func TestAddrLookupAndUpdate(t *testing.T) {
	assert := assert.New(t)

	d := NewDaemon(t, CmdTimeout(time.Second*90)).Start()
	defer d.ShutdownSuccess()

	minerPidForUpdate := core.RequireRandomPeerID()

	minerAddr := d.CreateMinerAddr()

	// capture original, pre-update miner pid
	lookupOutA := runSuccessFirstLine(d, "address", "lookup", minerAddr.String())

	// update the miner's peer ID
	updateMsg := runSuccessFirstLine(d,
		"miner", "update-peerid",
		minerAddr.String(),
		minerPidForUpdate.Pretty(),
	)

	// ensure mining happens after update message gets included in mempool
	d.RunSuccess("mpool --wait-for-count=1")
	d.RunSuccess("mining once")

	// wait for message to be included in a block
	d.WaitForMessageRequireSuccess(core.MustDecodeCid(updateMsg))

	// use the address lookup command to ensure update happened
	lookupOutB := runSuccessFirstLine(d, "address", "lookup", minerAddr.String())
	assert.Equal(minerPidForUpdate.Pretty(), lookupOutB)
	assert.NotEqual(lookupOutA, lookupOutB)
}
