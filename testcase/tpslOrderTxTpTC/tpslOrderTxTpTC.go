package tpslOrderTxTpTC

import (
	"log"
	"math/big"
	"math/rand"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/core/orderbook"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/kaiachain/kaia-load-tester/klayslave/account"
	"github.com/kaiachain/kaia-load-tester/klayslave/clipool"
	"github.com/myzhan/boomer"
)

const Name = "tpslOrderTxTpTC"

var (
	endPoint string
	nAcc     int
	accGrp   []*account.Account
	cliPool  clipool.ClientPool

	cursor uint32

	// User settings
	baseToken  = "3"
	quoteToken = "2" // USDT
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
}

func Run() {
	cli := cliPool.Alloc().(*ethclient.Client)
	defer cliPool.Free(cli)

	from := accGrp[atomic.AddUint32(&cursor, 1)%uint32(nAcc)]

	start := boomer.Now()
	err := SendRandomTx(cli, from)
	elapsed := boomer.Now() - start

	if err == nil {
		boomer.RecordSuccess("http", "SendNewOrderTxWithTpsl"+" to "+endPoint, elapsed, int64(10))
	} else {
		boomer.RecordFailure("http", "SendNewOrderTxWithTpsl"+" to "+endPoint, elapsed, err.Error())
	}
}

func SendRandomTx(cli *ethclient.Client, from *account.Account) error {
	var (
		side      orderbook.Side
		price     *big.Int
		quantity  *big.Int
		tpLimit   *big.Int
		slTrigger *big.Int
		tx        *types.Transaction
		orderType = orderbook.LIMIT
		err       error
		txType    int
	)

	// TPSL order scenario implementation (probability in parenthesis):
	// - tx1: TPSL order $3 BUY tpLimit=$4 (33.3%)
	// - tx2: Limit order $3 SELL (33.3%)
	// - tx3: limit order $4 BUY (33.3%)
	switch txType = rand.Intn(3); txType {
	case 0:
		side = orderbook.BUY
		price = scaleUp(3)
		quantity = scaleUp(1)
		tpLimit = scaleUp(4)
		slTrigger = scaleUp(1) // not triggered
		tx, err = from.GenNewOrderTxWithTpsl(baseToken, quoteToken, side, price, quantity, orderType, tpLimit, slTrigger, nil)
		if err != nil {
			log.Printf("Failed to generate new TPSL order tx (type%d): error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
				txType, err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
			return err
		}
	case 1:
		side = orderbook.SELL
		price = scaleUp(3)
		quantity = scaleUp(1)
		tx, err = from.GenNewOrderTx(baseToken, quoteToken, side, price, quantity, orderType)
		if err != nil {
			log.Printf("Failed to generate new order tx (type%d): error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
				txType, err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
			return err
		}
	case 2:
		side = orderbook.BUY
		price = scaleUp(4)
		quantity = scaleUp(1)
		tx, err = from.GenNewOrderTx(baseToken, quoteToken, side, price, quantity, orderType)
		if err != nil {
			log.Printf("Failed to generate new order tx (type%d): error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
				txType, err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
			return err
		}
	}

	_, err = from.SendTx(cli, tx)
	if err != nil {
		log.Printf("Failed to send new order tx (type%d): error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d\n",
			txType, err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
	}
	return err
}

func scaleUp(x int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(x), big.NewInt(1e18))
}
