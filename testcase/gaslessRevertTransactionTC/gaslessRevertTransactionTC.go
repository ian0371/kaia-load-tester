package gaslessRevertTransactionTC

import (
	"log"
	"math/big"
	"math/rand"

	"github.com/kaiachain/kaia-load-tester/klayslave/account"
	"github.com/kaiachain/kaia-load-tester/klayslave/clipool"
	"github.com/kaiachain/kaia/client"
	"github.com/myzhan/boomer"
)

const Name = "gaslessRevertTransactionTC"

var (
	endPoint string
	nAcc     int
	accGrp   []*account.Account
	cliPool  clipool.ClientPool

	TestTokenAccount *account.Account
	GsrAccount       *account.Account
)

func Init(accs []*account.Account, contractsParam []*account.Account, endpoint string, gp *big.Int) {
	endPoint = endpoint
	TestTokenAccount = contractsParam[account.ContractGaslessToken]
	GsrAccount = contractsParam[account.ContractGaslessSwapRouter]

	cliCreate := func() interface{} {
		c, err := client.Dial(endPoint)
		if err != nil {
			log.Fatalf("Failed to connect RPC: %v", err)
		}
		return c
	}

	cliPool.Init(20, 300, cliCreate)

	for _, acc := range accs {
		accGrp = append(accGrp, acc)
	}

	nAcc = len(accGrp)
}

func Run() {
	cli := cliPool.Alloc().(*client.Client)

	from := accGrp[rand.Int()%nAcc]
	testRecordName := "TransferNewGaslessRevertTx" + " to " + endPoint

	start := boomer.Now()

	_, _, _, err := from.TransferNewGaslessTx(cli, endPoint, TestTokenAccount, GsrAccount)

	elapsed := boomer.Now() - start

	cliPool.Free(cli)

	if err != nil {
		boomer.RecordFailure("http", testRecordName, elapsed, err.Error())
	} else {
		boomer.RecordSuccess("http", testRecordName, elapsed, int64(10))
	}
}
