package limitOrderTxTC

import (
	"context"
	"log"
	"math/big"
	"math/rand"
	"slices"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/core/orderbook"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/kaiachain/kaia-load-tester/klayslave/account"
	"github.com/kaiachain/kaia-load-tester/klayslave/clipool"
	"github.com/myzhan/boomer"
)

const Name = "limitOrderTxTC"

var (
	endPoint string
	nAcc     int
	accGrp   []*account.Account
	cliPool  clipool.ClientPool

	cursor uint32

	// LP settings
	askLiquidityPrice = scaleUp(3)
	bidLiquidityPrice = scaleUp(2)
	minQuantity       = scaleUp(1e3)
	pollInterval      = 3 * time.Second // LP checks and fills liquidity every pollInterval

	// User settings
	baseToken  = "2"
	quoteToken = "3"
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

	var (
		from     = accGrp[atomic.AddUint32(&cursor, 1)%uint32(nAcc)]
		quantity = scaleUp(1)
		side     = orderbook.Side(rand.Intn(2))
		price    = scaleUp(3 - int64(side)) // If buy, $3. If sell, $2.
	)

	start := boomer.Now()
	tx, err := from.GenNewOrderTx(baseToken, quoteToken, side, price, quantity, orderType)
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

		// skip check
		// deficits := checkLiquidityDeficit(cli)
		// askDeficit, bidDeficit := deficits[0], deficits[1]
		askDeficit, bidDeficit := minQuantity, minQuantity

		if askDeficit.Sign() > 0 {
			tx, err := from.GenNewOrderTx(baseToken, quoteToken, orderbook.SELL, askLiquidityPrice, askDeficit, orderbook.LIMIT)
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
			tx, err := from.GenNewOrderTx(baseToken, quoteToken, orderbook.BUY, bidLiquidityPrice, bidDeficit, orderbook.LIMIT)
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

		time.Sleep(pollInterval)
	}
}

// checkLiquidityDeficit returns [askDeficit, bidDeficit]
func checkLiquidityDeficit(cli *ethclient.Client) []*big.Int {
	c := cli.Client()
	var aggs []*orderbook.Aggregated
	c.CallContext(context.Background(), &aggs, "debug_getLvl2Data")

	symbol := baseToken + "/" + quoteToken
	askQuantity := findQuantity(aggs, symbol, askLiquidityPrice, orderbook.SELL)
	bidQuantity := findQuantity(aggs, symbol, bidLiquidityPrice, orderbook.BUY)

	askDeficit := new(big.Int).Sub(minQuantity, askQuantity)
	if askDeficit.Sign() < 0 {
		askDeficit = big.NewInt(0)
	}

	bidDeficit := new(big.Int).Sub(minQuantity, bidQuantity)
	if bidDeficit.Sign() < 0 {
		bidDeficit = big.NewInt(0)
	}

	return []*big.Int{askDeficit, bidDeficit}
}

func findQuantity(aggs []*orderbook.Aggregated, symbol string, price *big.Int, side orderbook.Side) *big.Int {
	aggIdx := slices.IndexFunc(aggs, func(a *orderbook.Aggregated) bool {
		return a.Symbol == symbol
	})
	if aggIdx == -1 {
		log.Printf("Symbol not found: %s. Regarding quantity as zero.", symbol)
		return big.NewInt(0)
	}

	agg := aggs[aggIdx]

	var arr [][]string
	if side == orderbook.SELL {
		arr = agg.Asks
	} else if side == orderbook.BUY {
		arr = agg.Bids
	} else {
		log.Printf("Invalid side: %d. Regarding quantity as zero.", side)
		return big.NewInt(0)
	}

	arrIdx := slices.IndexFunc(arr, func(a []string) bool {
		s, _ := strconv.ParseInt(a[0], 10, 64)
		p := scaleUp(s)
		return p.Cmp(price) == 0
	})
	if arrIdx == -1 {
		log.Printf("Price not found: %s. Regarding quantity as zero.", price.String())
		return big.NewInt(0)
	}

	quantity, _ := strconv.ParseInt(arr[arrIdx][1], 10, 64)
	return scaleUp(quantity)
}

func scaleUp(x int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(x), big.NewInt(1e18))
}
