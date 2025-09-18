package limitOrderFewTxTC

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

const Name = "limitOrderFewTxTC"

var (
	endPoint string
	nAcc     int
	accGrp   []*account.Account
	cliPool  clipool.ClientPool

	cursor uint32

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

	cli := cliPool.Alloc().(*ethclient.Client)
	provideInitialLiquidity(cli)
	go liquidityProvider(cli)
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

func provideInitialLiquidity(cli *ethclient.Client) {
	var (
		askLiquidityPrice = scaleUp(3)
		bidLiquidityPrice = scaleUp(2)
		initialQuantity   = scaleUp(1e6)
		splitCount        = 1 // How many orders should LP make for each liquidity provision
		from              = accGrp[atomic.AddUint32(&cursor, 1)%uint32(nAcc)]
	)

	provideLiquidity(cli, from, orderbook.SELL, askLiquidityPrice, initialQuantity, splitCount)
	provideLiquidity(cli, from, orderbook.BUY, bidLiquidityPrice, initialQuantity, splitCount)
}

// lp watches order status, and provides liquidity when liquidity is needed.
// liquidity orders: BUY at $2, SELL at $3
func liquidityProvider(cli *ethclient.Client) {
	var (
		askLiquidityPrice = scaleUp(3)
		bidLiquidityPrice = scaleUp(2)
		minQuantity       = scaleUp(1e6)
		splitCount        = 1                 // How many orders should LP make for each liquidity provision
		pollInterval      = 100 * time.Second // LP checks and fills liquidity every pollInterval
		fillDeficitOnly   = false             // If true, LP checks deficit from orderbook status. If false, minQuantity is filled.
	)

	for {
		var (
			from = accGrp[atomic.AddUint32(&cursor, 1)%uint32(nAcc)]
		)

		askDeficit, bidDeficit := minQuantity, minQuantity
		if fillDeficitOnly {
			deficits := checkLiquidityDeficit(cli, askLiquidityPrice, bidLiquidityPrice, minQuantity)
			askDeficit, bidDeficit = deficits[0], deficits[1]
		}

		if askDeficit.Sign() > 0 {
			provideLiquidity(cli, from, orderbook.SELL, askLiquidityPrice, askDeficit, splitCount)
			log.Printf("Sent ask side LP order: price=%s, quantity=%s", askLiquidityPrice.String(), askDeficit.String())
		}
		if bidDeficit.Sign() > 0 {
			provideLiquidity(cli, from, orderbook.BUY, bidLiquidityPrice, bidDeficit, splitCount)
			log.Printf("Sent bid side LP order: price=%s, quantity=%s", bidLiquidityPrice.String(), bidDeficit.String())
		}

		time.Sleep(pollInterval)
	}
}

func provideLiquidity(cli *ethclient.Client, from *account.Account, side orderbook.Side, price *big.Int, quantity *big.Int, splitCount int) {
	q := new(big.Int).Div(quantity, big.NewInt(int64(splitCount)))
	for i := 0; i < splitCount; i++ {
		tx, err := from.GenNewOrderTx(baseToken, quoteToken, side, price, q, orderbook.LIMIT)
		if err != nil {
			log.Printf("Failed to generate LP tx: error=%v, from=%s, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
				err, from.GetAddress().Hex(), baseToken, quoteToken, side, price.String(), q.String(), orderbook.LIMIT)
		}
		_, err = from.SendTx(cli, tx)
		if err != nil {
			log.Printf("Failed to send LP tx: error=%v, from=%s, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
				err, from.GetAddress().Hex(), baseToken, quoteToken, side, price.String(), q.String(), orderbook.LIMIT)
		}
	}
}

// checkLiquidityDeficit returns [askDeficit, bidDeficit]
func checkLiquidityDeficit(cli *ethclient.Client, askLiquidityPrice *big.Int, bidLiquidityPrice *big.Int, minQuantity *big.Int) []*big.Int {
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
