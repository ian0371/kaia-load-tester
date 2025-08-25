package dexTxSessionTC

import (
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/kaiachain/kaia-load-tester/klayslave/account"
	"github.com/kaiachain/kaia-load-tester/klayslave/clipool"
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
	/*
		cli := cliPool.Alloc().(*rpc.Client)

		from := accGrp[atomic.AddUint32(&cursor, 1)%uint32(nAcc)]
		to, value, input, _, err := CreateRandomArguments(from.GetAddress())
		if err != nil {
			fmt.Printf("Failed to creat arguments to send Legacy Tx: %v\n", err.Error())
			return
		}

		start := boomer.Now()

		txHashes, _, err := from.TransferNewLegacyTxWithEthBatch(cli, endPoint, to, value, input)

		elapsed := boomer.Now() - start

		if err != nil {
			boomer.Events.Publish("request_failure", "http", "TransferNewLegacyTx"+" to "+endPoint, elapsed, err.Error())
		}

		cliPool.Free(cli)

		for range txHashes {
			if err == nil {
				boomer.RecordSuccess("http", "TransferNewLegacyTx"+" to "+endPoint, elapsed, int64(10))
			} else {
				boomer.RecordFailure("http", "TransferNewLegacyTx"+" to "+endPoint, elapsed, err.Error())
			}
		}
	*/

	/*
		// Check test result with CheckResult function
		go func(transactionHashes []common.Hash) {
			receipts, err := from.CheckReceiptsBatch(cli, txHashes)
			if err != nil {
				fmt.Printf("Failed to get transaction receipts: %v\n", err.Error())
				for range txHashes {
					boomer.RecordFailure("http", "TransferNewLegacyTx"+" to "+endPoint, elapsed, err.Error())
				}
				return
			}

			for _, rc := range receipts {
				if rc.Status != types.ReceiptStatusSuccessful {
					boomer.RecordFailure("http", "TransferNewLegacyTx"+" to "+endPoint, elapsed, err.Error())
					continue
				}

				boomer.RecordSuccess("http", "TransferNewLegacyTx"+" to "+endPoint, elapsed, int64(10))
			}
		}(txHashes)
	*/
}

func CreateRandomArguments(addr common.Address) (*account.Account, *big.Int, string, int, error) {
	x := types.SessionContext{
		Command: types.SessionCreate,
		Session: types.Session{
			PublicKey: addr,
		},
	}
	fmt.Println(x)
	return nil, nil, "", 0, nil
}
