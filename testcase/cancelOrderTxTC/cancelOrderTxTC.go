package cancelOrderTxTC

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

const Name = "cancelOrderTxTC"

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
		boomer.RecordSuccess("http", "SendCancelOrderTx"+" to "+endPoint, elapsed, int64(10))
	} else {
		boomer.RecordFailure("http", "SendCancelOrderTx"+" to "+endPoint, elapsed, err.Error())
	}
}

func SendRandomTx(cli *ethclient.Client, from *account.Account) error {
	var (
		side      orderbook.Side
		price     *big.Int
		quantity  *big.Int
		tx        *types.Transaction
		orderType = orderbook.LIMIT
		err       error
		txType    int
	)

	// Cancel order scenario implementation (probability in parenthesis):
	// - tx1: create $1 BUY x 3 (40%)
	// - tx2: cancel all (10%)
	// - tx3: create $2 BUY (25%)
	// - tx4: create $2 SELL (25%)
	randNum := rand.Intn(100)

	switch {
	case randNum < 40:
		txType = 0
		side = orderbook.BUY
		price = scaleUp(1)
		quantity = scaleUp(1)
		tx, err = from.GenNewOrderTx(baseToken, quoteToken, side, price, quantity, orderType)
		if err != nil {
			log.Printf("Failed to generate new order tx (type%d): error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
				txType, err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
			return err
		}
	case randNum < 50: // 10% - tx2: cancel all
		txType = 1
		tx, err = from.GenCancelAllTx()
		if err != nil {
			log.Printf("Failed to generate cancel all orders tx (type%d): error=%v, baseToken=%s, quoteToken=%s",
				txType, err, baseToken, quoteToken)
			return err
		}
	case randNum < 75: // 25% - tx3: create $2 BUY
		txType = 2
		side = orderbook.BUY
		price = scaleUp(2)
		quantity = scaleUp(1)
		tx, err = from.GenNewOrderTx(baseToken, quoteToken, side, price, quantity, orderType)
		if err != nil {
			log.Printf("Failed to generate new order tx (type%d): error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
				txType, err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
			return err
		}
	default: // 25% - tx4: create $2 SELL
		txType = 3
		side = orderbook.SELL
		price = scaleUp(2)
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
		if txType == 1 { // cancel all orders
			log.Printf("Failed to send cancel all orders tx (type%d): error=%v, baseToken=%s, quoteToken=%s\n",
				txType, err, baseToken, quoteToken)
		} else { // new order
			log.Printf("Failed to send new order tx (type%d): error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d\n",
				txType, err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
		}
	}
	return err
}

func scaleUp(x int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(x), big.NewInt(1e18))
}
