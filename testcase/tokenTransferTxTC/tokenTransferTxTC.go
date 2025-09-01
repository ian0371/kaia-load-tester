package tokenTransferTxTC

import (
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/kaiachain/kaia-load-tester/klayslave/account"
	"github.com/kaiachain/kaia-load-tester/klayslave/clipool"
	"github.com/myzhan/boomer"
)

const Name = "tokenTransferTxTC"

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

	accGrp = append(accGrp, accs...)

	nAcc = len(accGrp)

	maxRetryCount = 30
}

func Run() {
	cli := cliPool.Alloc().(*ethclient.Client)
	defer cliPool.Free(cli)

	current := atomic.AddUint32(&cursor, 1)
	from := accGrp[current%uint32(nAcc)]
	to := accGrp[(current+1)%uint32(nAcc)]

	if len(from.GetSessionCtx()) == 0 {
		from.RegisterNewSession(cli)
	}

	start := boomer.Now()
	value := big.NewInt(int64(rand.Intn(5) + 1))
	tx, err := from.GenTokenTransferTx(to, value, "4")
	if err != nil {
		fmt.Printf("Failed to generate token transfer tx: %v\n", err.Error())
		return
	}
	_, err = from.SendTx(cli, tx)
	elapsed := boomer.Now() - start
	if err != nil {
		fmt.Printf("Failed to send token transfer tx: %v\n", err.Error())
		boomer.RecordFailure("http", "SendTransferTx"+" to "+endPoint, elapsed, err.Error())
		return
	}

	boomer.RecordSuccess("http", "SendTransferTx"+" to "+endPoint, elapsed, int64(10))
}
