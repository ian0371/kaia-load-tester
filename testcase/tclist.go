package testcase

import (
	"math/big"

	"github.com/kaiachain/kaia-load-tester/klayslave/account"
	"github.com/kaiachain/kaia-load-tester/testcase/limitOrderTxTC"
	"github.com/kaiachain/kaia-load-tester/testcase/sessionTxTC"
	"github.com/kaiachain/kaia-load-tester/testcase/tokenTransferTxTC"
	"github.com/kaiachain/kaia-load-tester/testcase/transferTxTC"
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
	sessionTxTC.Name: {
		Name:   sessionTxTC.Name,
		Weight: 10,
		Fn:     sessionTxTC.Run,
		Init:   sessionTxTC.Init,
	},
	transferTxTC.Name: {
		Name:   transferTxTC.Name,
		Weight: 10,
		Fn:     transferTxTC.Run,
		Init:   transferTxTC.Init,
	},
	tokenTransferTxTC.Name: {
		Name:   tokenTransferTxTC.Name,
		Weight: 10,
		Fn:     tokenTransferTxTC.Run,
		Init:   tokenTransferTxTC.Init,
	},
	limitOrderTxTC.Name: {
		Name:   limitOrderTxTC.Name,
		Weight: 10,
		Fn:     limitOrderTxTC.Run,
		Init:   limitOrderTxTC.Init,
	},
}
