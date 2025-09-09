package stopOrderTxTC

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

const Name = "stopOrderTxTC"

var (
	endPoint string
	nAcc     int
	accGrp   []*account.Account
	cliPool  clipool.ClientPool

	cursor uint32

	// User settings
	baseToken  = "2"
	quoteToken = "3"
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
		boomer.RecordSuccess("http", "SendNewStopOrderTx"+" to "+endPoint, elapsed, int64(10))
	} else {
		boomer.RecordFailure("http", "SendNewStopOrderTx"+" to "+endPoint, elapsed, err.Error())
	}
}

func SendRandomTx(cli *ethclient.Client, from *account.Account) error {
	var (
		side      orderbook.Side
		price     *big.Int
		stopPrice *big.Int
		quantity  *big.Int
		tx        *types.Transaction
		orderType = orderbook.LIMIT
		err       error
	)

	switch rand.Intn(4) {
	case 0:
		side = orderbook.BUY
		price = scaleUp(3)
		stopPrice = scaleUp(2)
		quantity = scaleUp(1)
		tx, err = from.GenNewStopOrderTx(baseToken, quoteToken, side, stopPrice, price, quantity, orderType)
		if err != nil {
			log.Printf("Failed to generate new stop order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
				err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
			return err
		}
	case 1:
		side = orderbook.BUY
		price = scaleUp(2)
		quantity = scaleUp(1)
		tx, err = from.GenNewOrderTx(baseToken, quoteToken, side, price, quantity, orderType)
		if err != nil {
			log.Printf("Failed to generate new order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
				err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
			return err
		}
	case 2:
		side = orderbook.SELL
		price = scaleUp(2)
		quantity = scaleUp(1)
		tx, err = from.GenNewOrderTx(baseToken, quoteToken, side, price, quantity, orderType)
		if err != nil {
			log.Printf("Failed to generate new order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
				err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
			return err
		}
	case 3:
		side = orderbook.SELL
		price = scaleUp(3)
		quantity = scaleUp(1)
		tx, err = from.GenNewOrderTx(baseToken, quoteToken, side, price, quantity, orderType)
		if err != nil {
			log.Printf("Failed to generate new order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
				err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
			return err
		}
	}

	_, err = from.SendTx(cli, tx)
	if err != nil {
		log.Printf("Failed to send new order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d\n",
			err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
	}
	return err
}

func scaleUp(x int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(x), big.NewInt(1e18))
}
