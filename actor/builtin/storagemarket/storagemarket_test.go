package storagemarket_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	. "github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageMarketCreateMiner(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	pid := core.RequireRandomPeerID()
	pdata := actor.MustConvertParams(types.NewBytesAmount(10000), []byte{}, pid)
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(100), "createMiner", pdata)
	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)
	require.Nil(result.ExecutionError)

	outAddr, err := types.NewAddressFromBytes(result.Receipt.Return[0])
	require.NoError(err)
	minerActor, err := st.GetActor(ctx, outAddr)
	require.NoError(err)

	storageMkt, err := st.GetActor(ctx, address.StorageMarketAddress)
	require.NoError(err)

	assert.Equal(types.NewAttoFILFromFIL(0), storageMkt.Balance)
	assert.Equal(types.NewAttoFILFromFIL(100), minerActor.Balance)

	var mstor miner.State
	builtin.RequireReadState(t, vms, outAddr, minerActor, &mstor)

	assert.Equal(mstor.Collateral, types.NewAttoFILFromFIL(100))
	assert.Equal(mstor.PledgeBytes, types.NewBytesAmount(10000))
	assert.Equal(mstor.PeerID, pid)
}

func TestStorageMarketCreateMinerPledgeTooLow(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	pdata := actor.MustConvertParams(types.NewBytesAmount(50), []byte{}, core.RequireRandomPeerID())
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(100), "createMiner", pdata)
	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	assert.NoError(err)
	assert.Contains(result.ExecutionError.Error(), Errors[ErrPledgeTooLow].Error())
}

func TestStorageMarkeCreateMinerDoesNotOverwriteActorBalance(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	// create account of future miner actor by sending FIL to the predicted address
	minerAddr, err := deriveMinerAddress(address.TestAddress, 0)
	require.NoError(err)

	msg := types.NewMessage(address.TestAddress2, minerAddr, 0, types.NewAttoFILFromFIL(100), "", []byte{})
	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)
	require.Equal(uint8(0), result.Receipt.ExitCode)

	pdata := actor.MustConvertParams(types.NewBytesAmount(15000), []byte{}, core.RequireRandomPeerID())
	msg = types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(200), "createMiner", pdata)
	result, err = core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)
	require.Equal(uint8(0), result.Receipt.ExitCode)
	require.NoError(result.ExecutionError)

	// ensure our derived address is the address storage market creates
	createdAddress, err := types.NewAddressFromBytes(result.Receipt.Return[0])
	require.NoError(err)
	assert.Equal(minerAddr, createdAddress)

	miner, err := st.GetActor(ctx, minerAddr)
	require.NoError(err)

	// miner balance should be sum of messages
	assert.Equal(types.NewAttoFILFromFIL(300), miner.Balance)
}

func TestStorageMarkeCreateMinerErrorsOnInvalidKey(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	publicKey := []byte("012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567")
	pdata := actor.MustConvertParams(types.NewBytesAmount(15000), publicKey, core.RequireRandomPeerID())
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(200), "createMiner", pdata)
	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	require.NoError(err)
	assert.Contains(result.ExecutionError.Error(), miner.Errors[miner.ErrPublicKeyTooBig].Error())
}

func TestStorageMarketAddBid(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, vms := core.CreateStorages(ctx, t)

	// create a bid
	pdata := actor.MustConvertParams(types.NewAttoFILFromFIL(20), types.NewBytesAmount(30))
	msg := types.NewMessage(address.TestAddress, address.StorageMarketAddress, 0, types.NewAttoFILFromFIL(600), "addBid", pdata)
	result, err := core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	assert.NoError(err)

	assert.Equal(uint8(0), result.Receipt.ExitCode)
	assert.Equal(big.NewInt(0), big.NewInt(0).SetBytes(result.Receipt.Return[0]))

	// create another bid
	pdata = actor.MustConvertParams(types.NewAttoFILFromFIL(15), types.NewBytesAmount(80))
	msg = types.NewMessage(address.TestAddress, address.StorageMarketAddress, 1, types.NewAttoFILFromFIL(1200), "addBid", pdata)
	result, err = core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	assert.NoError(err)

	assert.Equal(uint8(0), result.Receipt.ExitCode)
	assert.Equal(big.NewInt(1), big.NewInt(0).SetBytes(result.Receipt.Return[0]))

	// try to create a bid, but send wrong value
	pdata = actor.MustConvertParams(types.NewAttoFILFromFIL(90), types.NewBytesAmount(100))
	msg = types.NewMessage(address.TestAddress, address.StorageMarketAddress, 2, types.NewAttoFILFromFIL(600), "addBid", pdata)
	result, err = core.ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	assert.NoError(err)
	assert.Contains(result.ExecutionError.Error(), "must send price * size funds to create bid")
}

// this is used to simulate an attack where someone derives the likely address of another miner's
// minerActor and sends some FIL. If that FIL creates an actor tha cannot be upgraded to a miner
// actor, this action will block the other user. Another possibility is that the miner actor will
// overwrite the account with the balance thereby obliterating the FIL.
func deriveMinerAddress(creator types.Address, nonce uint64) (types.Address, error) {
	buf := new(bytes.Buffer)

	if _, err := buf.Write(creator.Bytes()); err != nil {
		return types.Address{}, err
	}

	if err := binary.Write(buf, binary.BigEndian, nonce); err != nil {
		return types.Address{}, err
	}

	hash := types.AddressHash(buf.Bytes())

	return types.NewMainnetAddress(hash), nil
}
