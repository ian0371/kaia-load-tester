package testcase

import (
	"math/big"

	"github.com/kaiachain/kaia-load-tester/klayslave/account"
	"github.com/kaiachain/kaia-load-tester/testcase/dexTxSessionTC"
)

type ExtendedTask struct {
	Name   string
	Weight int
	Fn     func()
	Init   func(accs []*account.Account, endpoint string, gp *big.Int)
}
type ExtendedTaskSet []*ExtendedTask

// TcList initializes TCs and returns a slice of TCs.
var TcList = map[string]*ExtendedTask{
	"sessionTxTC": {
		Name:   "sessionTxTC",
		Weight: 10,
		Fn:     dexTxSessionTC.Run,
		Init:   dexTxSessionTC.Init,
	},
}
