package dexTxSessionTC

import (
	"fmt"
	"log"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/kaiachain/kaia-load-tester/klayslave/account"
	"github.com/kaiachain/kaia-load-tester/klayslave/clipool"
	"github.com/myzhan/boomer"
)

const Name = "dexTxSessionTC"

var (
	endPoint string
	nAcc     int
	accGrp   []*account.Account
	cliPool  clipool.ClientPool

	maxRetryCount int

	cursor uint32
)

func Init(accs []*account.Account, endpoint string, _ *big.Int) {
	endPoint = endpoint

	cliCreate := func() any {
		c, err := ethclient.Dial(endPoint)
		if err != nil {
			log.Fatalf("Failed to connect RPC: %v", err)
		}
		return c
	}

	cliPool.Init(1000, 3000, cliCreate)

	for _, acc := range accs {
		accGrp = append(accGrp, acc)
	}

	nAcc = len(accGrp)

	maxRetryCount = 30
}

func Run() {
	cli := cliPool.Alloc().(*ethclient.Client)
	defer cliPool.Free(cli)

	from := accGrp[atomic.AddUint32(&cursor, 1)%uint32(nAcc)]

	// create session
	sessionCreatetx, sessionCtx, sessionKey, err := from.GenSessionCreateTx()
	if err != nil {
		return
	}
	sessionDeleteTx, err := from.GenSessionDeleteTx(sessionCtx, sessionKey)
	if err != nil {
		return
	}
	txs := []*types.Transaction{sessionCreatetx, sessionDeleteTx}
	start := boomer.Now()
	hashes, err := from.SendTxBatch(cli, txs)
	elapsed := boomer.Now() - start
	if err != nil {
		fmt.Printf("Failed to send session tx: %v\n", err.Error())
		boomer.RecordFailure("http", "SendSessionTx"+" to "+endPoint, elapsed, err.Error())
		return
	}

	boomer.RecordSuccess("http", "SendSessionTx"+" to "+endPoint, elapsed, int64(10))

	for i, hash := range hashes {
		if hash == (common.Hash{}) {
			fmt.Printf("Failed to send session tx %v: %v\n", txs[i], err.Error())
			boomer.RecordFailure("http", "SendSessionTx"+" to "+endPoint, elapsed, err.Error())
		} else {
			boomer.RecordSuccess("http", "SendSessionTx"+" to "+endPoint, elapsed, int64(10))
		}
	}
}
