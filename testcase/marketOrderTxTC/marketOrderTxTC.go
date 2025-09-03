package marketOrderTxTC

import (
	"context"
	"log"
	"math/big"
	"math/rand"
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

const Name = "marketOrderTxTC"

var (
	endPoint string
	nAcc     int
	accGrp   []*account.Account
	cliPool  clipool.ClientPool

	cursor uint32

	// LP settings
	askLiquidityPrice *uint256.Int = uint256.NewInt(uint64(3e18))
	bidLiquidityPrice *uint256.Int = uint256.NewInt(uint64(2e18))
	minQuantity       *uint256.Int = uint256.NewInt(uint64(100000))

	// User settings
	baseToken  = "2"
	quoteToken = "3"
	base       = uint256.NewInt(uint64(1e18))
	orderType  = orderbook.MARKET
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
		from           = accGrp[atomic.AddUint32(&cursor, 1)%uint32(nAcc)]
		priceOffset    = uint256.NewInt(uint64(rand.Intn(5) + 3))
		quantityOffset = uint256.NewInt(uint64(rand.Intn(5) + 3))
		price          = new(uint256.Int).Mul(base, priceOffset)
		quantity       = new(uint256.Int).Mul(base, quantityOffset)
		side           = orderbook.Side(rand.Intn(2))
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
					err, from.GetAddress().Hex(), baseToken, quoteToken, orderbook.SELL, askLiquidityPrice.String(), askDeficit.String(), orderbook.MARKET)
			}
			_, err = from.SendTx(cli, tx)
			if err != nil {
				log.Printf("Failed to send LP tx: error=%v, from=%s, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
					err, from.GetAddress().Hex(), baseToken, quoteToken, orderbook.SELL, askLiquidityPrice.String(), askDeficit.String(), orderbook.MARKET)
			}
		}
		if bidDeficit.Sign() > 0 {
			tx, err := from.GenNewOrderTx(baseToken, quoteToken, orderbook.BUY, bidLiquidityPrice.ToBig(), bidDeficit.ToBig(), orderbook.MARKET)
			if err != nil {
				log.Printf("Failed to generate LP tx: error=%v, from=%s, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
					err, from.GetAddress().Hex(), baseToken, quoteToken, orderbook.BUY, bidLiquidityPrice.String(), bidDeficit.String(), orderbook.MARKET)
			}
			_, err = from.SendTx(cli, tx)
			if err != nil {
				log.Printf("Failed to send LP tx: error=%v, from=%s, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
					err, from.GetAddress().Hex(), baseToken, quoteToken, orderbook.BUY, bidLiquidityPrice.String(), bidDeficit.String(), orderbook.MARKET)
			}
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

func findQuantity(aggs []*orderbook.Aggregated, token string, price *uint256.Int, side orderbook.Side) *uint256.Int {
	aggIdx := slices.IndexFunc(aggs, func(a *orderbook.Aggregated) bool {
		return a.Symbol == token
	})
	if aggIdx == -1 {
		return uint256.NewInt(0)
	}

	agg := aggs[aggIdx]

	var arr [][]string
	if side == orderbook.BUY {
		arr = agg.Asks
	} else if side == orderbook.SELL {
		arr = agg.Bids
	} else {
		log.Printf("Invalid side: %d", side)
		return uint256.NewInt(0)
	}

	arrIdx := slices.IndexFunc(arr, func(a []string) bool {
		p, _ := uint256.FromBig(new(big.Int).SetBytes([]byte(a[0])))
		return p.Cmp(price) == 0
	})
	if arrIdx == -1 {
		return uint256.NewInt(0)
	}

	quantity, _ := uint256.FromDecimal(arr[arrIdx][1])
	return quantity
}
