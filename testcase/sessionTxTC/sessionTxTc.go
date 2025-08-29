package sessionTxTC

import (
	"fmt"
	"log"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/kaiachain/kaia-load-tester/klayslave/account"
	"github.com/kaiachain/kaia-load-tester/klayslave/clipool"
	"github.com/myzhan/boomer"
)

const Name = "sessionTxTC"

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
	rets, err := from.SendTxBatch(cli, txs)
	elapsed := boomer.Now() - start
	if err != nil {
		fmt.Printf("Failed to send session tx: %v\n", err.Error())
		return
	}

	for _, ret := range rets {
		if ret != nil && len(*ret) == 32 {
			boomer.RecordSuccess("http", "SendSessionTx"+" to "+endPoint, elapsed, int64(10))
		} else {
			fmt.Printf("Failed to send session tx %v\n", ret.String())
			boomer.RecordFailure("http", "SendSessionTx"+" to "+endPoint, elapsed, ret.String())
		}
	}
}
