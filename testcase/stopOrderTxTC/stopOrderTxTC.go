package stopOrderTxTC

import (
	"context"
	"log"
	"math/big"
	"slices"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/core/orderbook"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/holiman/uint256"
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

	// LP settings
	askLiquidityPrice *uint256.Int = new(uint256.Int).Mul(base, uint256.NewInt(uint64(3)))   // $3
	bidLiquidityPrice *uint256.Int = new(uint256.Int).Mul(base, uint256.NewInt(uint64(2)))   // $2
	minQuantity       *uint256.Int = new(uint256.Int).Mul(base, uint256.NewInt(uint64(1e6))) // 1M

	// User settings
	baseToken  = "2"
	quoteToken = "3"
	base       = uint256.NewInt(uint64(1e18))
	orderType  = orderbook.LIMIT
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

	go liquidityProvider(cliPool.Alloc().(*ethclient.Client))
}

func Run() {
	cli := cliPool.Alloc().(*ethclient.Client)
	defer cliPool.Free(cli)

	RunBidStop(cli)
	RunAskStop(cli)
	RunBid(cli)
	RunAsk(cli)
}

func RunBidStop(cli *ethclient.Client) {
	var (
		from      = accGrp[atomic.AddUint32(&cursor, 1)%uint32(nAcc)]
		stopPrice = new(uint256.Int).Mul(base, uint256.NewInt(3))
		price     = new(uint256.Int).Mul(base, uint256.NewInt(2))
		quantity  = new(uint256.Int).Mul(base, uint256.NewInt(uint64(100)))
		side      = orderbook.BUY
	)

	start := boomer.Now()
	tx, err := from.GenNewStopOrderTx(baseToken, quoteToken, side, stopPrice.ToBig(), price.ToBig(), quantity.ToBig(), orderType)
	if err != nil {
		log.Printf("Failed to generate new stop order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
			err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
		return
	}
	_, err = from.SendTx(cli, tx)
	elapsed := boomer.Now() - start
	if err != nil {
		log.Printf("Failed to send new stop order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d\n",
			err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
		boomer.RecordFailure("http", "SendNewOrderTx"+" to "+endPoint, elapsed, err.Error())
		return
	}

	boomer.RecordSuccess("http", "SendNewOrderTx"+" to "+endPoint, elapsed, int64(10))
}

func RunAskStop(cli *ethclient.Client) {
	var (
		from      = accGrp[atomic.AddUint32(&cursor, 1)%uint32(nAcc)]
		stopPrice = new(uint256.Int).Mul(base, uint256.NewInt(2))
		price     = new(uint256.Int).Mul(base, uint256.NewInt(3))
		quantity  = new(uint256.Int).Mul(base, uint256.NewInt(uint64(100)))
		side      = orderbook.SELL
	)

	start := boomer.Now()
	tx, err := from.GenNewStopOrderTx(baseToken, quoteToken, side, stopPrice.ToBig(), price.ToBig(), quantity.ToBig(), orderType)
	if err != nil {
		log.Printf("Failed to generate new stop order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
			err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
		return
	}
	_, err = from.SendTx(cli, tx)
	elapsed := boomer.Now() - start
	if err != nil {
		log.Printf("Failed to send new stop order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d\n",
			err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
		boomer.RecordFailure("http", "SendNewOrderTx"+" to "+endPoint, elapsed, err.Error())
		return
	}

	boomer.RecordSuccess("http", "SendNewOrderTx"+" to "+endPoint, elapsed, int64(10))
}

func RunBid(cli *ethclient.Client) {
	var (
		from     = accGrp[atomic.AddUint32(&cursor, 1)%uint32(nAcc)]
		price    = new(uint256.Int).Mul(base, uint256.NewInt(2))
		quantity = new(uint256.Int).Mul(base, uint256.NewInt(uint64(1)))
		side     = orderbook.BUY
	)

	start := boomer.Now()
	tx, err := from.GenNewOrderTx(baseToken, quoteToken, side, price.ToBig(), quantity.ToBig(), orderType)
	if err != nil {
		log.Printf("Failed to generate new order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
			err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
		return
	}
	_, err = from.SendTx(cli, tx)
	elapsed := boomer.Now() - start
	if err != nil {
		log.Printf("Failed to send new order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d\n",
			err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
		boomer.RecordFailure("http", "SendNewOrderTx"+" to "+endPoint, elapsed, err.Error())
		return
	}

	boomer.RecordSuccess("http", "SendNewOrderTx"+" to "+endPoint, elapsed, int64(10))
}

func RunAsk(cli *ethclient.Client) {
	var (
		from     = accGrp[atomic.AddUint32(&cursor, 1)%uint32(nAcc)]
		price    = new(uint256.Int).Mul(base, uint256.NewInt(3))
		quantity = new(uint256.Int).Mul(base, uint256.NewInt(uint64(1)))
		side     = orderbook.SELL
	)

	start := boomer.Now()
	tx, err := from.GenNewOrderTx(baseToken, quoteToken, side, price.ToBig(), quantity.ToBig(), orderType)
	if err != nil {
		log.Printf("Failed to generate new order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
			err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
		return
	}
	_, err = from.SendTx(cli, tx)
	elapsed := boomer.Now() - start
	if err != nil {
		log.Printf("Failed to send new order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d\n",
			err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
		boomer.RecordFailure("http", "SendNewOrderTx"+" to "+endPoint, elapsed, err.Error())
		return
	}

	boomer.RecordSuccess("http", "SendNewOrderTx"+" to "+endPoint, elapsed, int64(10))
}

// lp watches order status, and provides liquidity when liquidity is needed.
// liquidity orders: BUY at $2, SELL at $3
func liquidityProvider(cli *ethclient.Client) {
	for {
		var (
			from = accGrp[atomic.AddUint32(&cursor, 1)%uint32(nAcc)]
		)

		deficits := checkLiquidityDeficit(cli)
		askDeficit, bidDeficit := deficits[0], deficits[1]

		if askDeficit.Sign() > 0 {
			tx, err := from.GenNewOrderTx(baseToken, quoteToken, orderbook.SELL, askLiquidityPrice.ToBig(), askDeficit.ToBig(), orderbook.LIMIT)
			if err != nil {
				log.Printf("Failed to generate LP tx: error=%v, from=%s, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
					err, from.GetAddress().Hex(), baseToken, quoteToken, orderbook.SELL, askLiquidityPrice.String(), askDeficit.String(), orderbook.LIMIT)
			}
			_, err = from.SendTx(cli, tx)
			if err != nil {
				log.Printf("Failed to send LP tx: error=%v, from=%s, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
					err, from.GetAddress().Hex(), baseToken, quoteToken, orderbook.SELL, askLiquidityPrice.String(), askDeficit.String(), orderbook.LIMIT)
			}
			log.Printf("Sent ask side LP order: price=%s, quantity=%s", askLiquidityPrice.String(), askDeficit.String())
		}
		if bidDeficit.Sign() > 0 {
			tx, err := from.GenNewOrderTx(baseToken, quoteToken, orderbook.BUY, bidLiquidityPrice.ToBig(), bidDeficit.ToBig(), orderbook.LIMIT)
			if err != nil {
				log.Printf("Failed to generate LP tx: error=%v, from=%s, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
					err, from.GetAddress().Hex(), baseToken, quoteToken, orderbook.BUY, bidLiquidityPrice.String(), bidDeficit.String(), orderbook.LIMIT)
			}
			_, err = from.SendTx(cli, tx)
			if err != nil {
				log.Printf("Failed to send LP tx: error=%v, from=%s, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
					err, from.GetAddress().Hex(), baseToken, quoteToken, orderbook.BUY, bidLiquidityPrice.String(), bidDeficit.String(), orderbook.LIMIT)
			}
			log.Printf("Sent bid side LP order: price=%s, quantity=%s", bidLiquidityPrice.String(), bidDeficit.String())
		}

		time.Sleep(10 * time.Second)
	}
}

// checkLiquidityDeficit returns [askDeficit, bidDeficit]
func checkLiquidityDeficit(cli *ethclient.Client) []*uint256.Int {
	c := cli.Client()
	var aggs []*orderbook.Aggregated
	c.CallContext(context.Background(), &aggs, "debug_getLvl2DataFromLvl3")

	symbol := baseToken + "/" + quoteToken
	askQuantity := findQuantity(aggs, symbol, askLiquidityPrice, orderbook.SELL)
	bidQuantity := findQuantity(aggs, symbol, bidLiquidityPrice, orderbook.BUY)

	askDeficit := new(uint256.Int).Sub(minQuantity, askQuantity)
	if askDeficit.Sign() < 0 {
		askDeficit = uint256.NewInt(0)
	}

	bidDeficit := new(uint256.Int).Sub(minQuantity, bidQuantity)
	if bidDeficit.Sign() < 0 {
		bidDeficit = uint256.NewInt(0)
	}

	return []*uint256.Int{askDeficit, bidDeficit}
}

func findQuantity(aggs []*orderbook.Aggregated, symbol string, price *uint256.Int, side orderbook.Side) *uint256.Int {
	aggIdx := slices.IndexFunc(aggs, func(a *orderbook.Aggregated) bool {
		return a.Symbol == symbol
	})
	if aggIdx == -1 {
		log.Printf("Symbol not found: %s. Regarding quantity as zero.", symbol)
		return uint256.NewInt(0)
	}

	agg := aggs[aggIdx]

	var arr [][]string
	if side == orderbook.SELL {
		arr = agg.Asks
	} else if side == orderbook.BUY {
		arr = agg.Bids
	} else {
		log.Printf("Invalid side: %d. Regarding quantity as zero.", side)
		return uint256.NewInt(0)
	}

	arrIdx := slices.IndexFunc(arr, func(a []string) bool {
		p, _ := uint256.FromDecimal(a[0])
		p = new(uint256.Int).Mul(base, p)
		return p.Cmp(price) == 0
	})
	if arrIdx == -1 {
		log.Printf("Price not found: %s. Regarding quantity as zero.", price.String())
		return uint256.NewInt(0)
	}

	quantity, _ := uint256.FromDecimal(arr[arrIdx][1])
	quantity = new(uint256.Int).Mul(base, quantity)
	return quantity
}
