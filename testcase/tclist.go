package testcase

import (
	"math/big"

	"github.com/kaiachain/kaia-load-tester/klayslave/account"
	"github.com/kaiachain/kaia-load-tester/testcase/cancelOrderTxTC"
	"github.com/kaiachain/kaia-load-tester/testcase/ethLegacyTxTC"
	"github.com/kaiachain/kaia-load-tester/testcase/limitOrderLPTxTC"
	"github.com/kaiachain/kaia-load-tester/testcase/limitOrderTxTC"
	"github.com/kaiachain/kaia-load-tester/testcase/marketOrderTxTC"
	"github.com/kaiachain/kaia-load-tester/testcase/sessionTxTC"
	"github.com/kaiachain/kaia-load-tester/testcase/stopOrderTxTC"
	"github.com/kaiachain/kaia-load-tester/testcase/tokenTransferTxTC"
	"github.com/kaiachain/kaia-load-tester/testcase/tpslOrderTxSlTC"
	"github.com/kaiachain/kaia-load-tester/testcase/tpslOrderTxTpTC"
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
	ethLegacyTxTC.Name: {
		Name:   ethLegacyTxTC.Name,
		Weight: 10,
		Fn:     ethLegacyTxTC.Run,
		Init:   ethLegacyTxTC.Init,
	},
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
	limitOrderLPTxTC.Name: {
		Name:   limitOrderLPTxTC.Name,
		Weight: 10,
		Fn:     limitOrderLPTxTC.Run,
		Init:   limitOrderLPTxTC.Init,
	},
	marketOrderTxTC.Name: {
		Name:   marketOrderTxTC.Name,
		Weight: 10,
		Fn:     marketOrderTxTC.Run,
		Init:   marketOrderTxTC.Init,
	},
	stopOrderTxTC.Name: {
		Name:   stopOrderTxTC.Name,
		Weight: 10,
		Fn:     stopOrderTxTC.Run,
		Init:   stopOrderTxTC.Init,
	},
	tpslOrderTxTpTC.Name: {
		Name:   tpslOrderTxTpTC.Name,
		Weight: 10,
		Fn:     tpslOrderTxTpTC.Run,
		Init:   tpslOrderTxTpTC.Init,
	},
	tpslOrderTxSlTC.Name: {
		Name:   tpslOrderTxSlTC.Name,
		Weight: 10,
		Fn:     tpslOrderTxSlTC.Run,
		Init:   tpslOrderTxSlTC.Init,
	},
	cancelOrderTxTC.Name: {
		Name:   cancelOrderTxTC.Name,
		Weight: 10,
		Fn:     cancelOrderTxTC.Run,
		Init:   cancelOrderTxTC.Init,
	},
}
