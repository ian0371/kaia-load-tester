package tpslOrderTxSlTC

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

const Name = "tpslOrderTxSlTC"

var (
	endPoint string
	nAcc     int
	accGrp   []*account.Account
	cliPool  clipool.ClientPool

	cursor uint32

	// LP settings
	phase1AskLiquidityPrice *uint256.Int = new(uint256.Int).Mul(base, uint256.NewInt(uint64(3)))   // $3
	phase1BidLiquidityPrice *uint256.Int = new(uint256.Int).Mul(base, uint256.NewInt(uint64(2)))   // $2
	phase2AskLiquidityPrice *uint256.Int = new(uint256.Int).Mul(base, uint256.NewInt(uint64(2)))   // $2
	phase2BidLiquidityPrice *uint256.Int = new(uint256.Int).Mul(base, uint256.NewInt(uint64(1)))   // $1
	minQuantity             *uint256.Int = new(uint256.Int).Mul(base, uint256.NewInt(uint64(1e6))) // 1M

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

	picker := atomic.AddUint32(&cursor, 1) % uint32(nAcc)
	for picker == 0 { // because 0 is LP
		picker = atomic.AddUint32(&cursor, 1) % uint32(nAcc)
	}

	var (
		from      = accGrp[picker]
		price     = new(uint256.Int).Mul(base, uint256.NewInt(3))
		quantity  = new(uint256.Int).Mul(base, uint256.NewInt(uint64(1)))
		side      = orderbook.BUY
		tpLimit   = new(uint256.Int).Mul(base, uint256.NewInt(9999)) // not triggered
		slTrigger = new(uint256.Int).Mul(base, uint256.NewInt(2))
		slLimit   = new(uint256.Int).Mul(base, uint256.NewInt(1))
	)

	start := boomer.Now()
	tx, err := from.GenNewOrderTxWithTpsl(baseToken, quoteToken, side, price.ToBig(), quantity.ToBig(), orderType, tpLimit.ToBig(), slTrigger.ToBig(), slLimit.ToBig())
	if err != nil {
		log.Printf("Failed to generate new TPSL order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
			err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
		return
	}
	_, err = from.SendTx(cli, tx)
	elapsed := boomer.Now() - start
	if err != nil {
		log.Printf("Failed to send new TPSL order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d\n",
			err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
		boomer.RecordFailure("http", "SendNewOrderTxWithTpsl"+" to "+endPoint, elapsed, err.Error())
		return
	}

	// $2 (slTrigger) LIMIT BUY
	tx, err = from.GenNewOrderTx(baseToken, quoteToken, side, slTrigger.ToBig(), quantity.ToBig(), orderType)
	if err != nil {
		log.Printf("Failed to generate new TPSL order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d",
			err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
		return
	}
	_, err = from.SendTx(cli, tx)
	elapsed = boomer.Now() - start
	if err != nil {
		log.Printf("Failed to send new order tx: error=%v, baseToken=%s, quoteToken=%s, side=%d, price=%s, quantity=%s, orderType=%d\n",
			err, baseToken, quoteToken, side, price.String(), quantity.String(), orderType)
		boomer.RecordFailure("http", "SendNewOrderTx"+" to "+endPoint, elapsed, err.Error())
		return
	}

	boomer.RecordSuccess("http", "SendNewOrderTx"+" to "+endPoint, elapsed, int64(10))
}

// phase alternates every `phaseDurationSec` seconds
func getPhase() int {
	const (
		phaseDurationSec = 30
		numPhase         = 2
	)

	if time.Now().UTC().Unix()/phaseDurationSec%numPhase == 0 {
		return 1
	} else {
		return 2
	}
}

// lp alternates between phase1 and phase2
// In phase 1, $2 BUY and $3 SELL
// In phase 2, $1 BUY and $2 SELL
func liquidityProvider(cli *ethclient.Client) {
	var currPhase, prevPhase int
	for {
		currPhase, prevPhase = getPhase(), currPhase
		if prevPhase != currPhase {
			cancelAllLpOrders(cli) // run only once when phase alternates
		}
		switch currPhase {
		case 1:
			provideLiquidity(cli, phase1AskLiquidityPrice, phase1BidLiquidityPrice)
		case 2:
			provideLiquidity(cli, phase2AskLiquidityPrice, phase2BidLiquidityPrice)
		}

		time.Sleep(2 * time.Second)
	}
}

func cancelAllLpOrders(cli *ethclient.Client) {
	from := accGrp[0]
	tx, err := from.GenNewCancelAllTx()
	if err != nil {
		log.Printf("Failed to generate cancel all tx: error=%v", err)
		return
	}
	_, err = from.SendTx(cli, tx)
	if err != nil {
		log.Printf("Failed to send cancel all tx: error=%v", err)
		return
	}
	log.Printf("Sent cancel all tx: %s", tx.Hash().Hex())
}

func provideLiquidity(cli *ethclient.Client, askLiquidityPrice *uint256.Int, bidLiquidityPrice *uint256.Int) {
	var (
		from                   = accGrp[0]
		deficits               = checkLiquidityDeficit(cli, askLiquidityPrice, bidLiquidityPrice)
		askDeficit, bidDeficit = deficits[0], deficits[1]
	)

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
}

// checkLiquidityDeficit returns [askDeficit, bidDeficit]
func checkLiquidityDeficit(cli *ethclient.Client, askLiquidityPrice *uint256.Int, bidLiquidityPrice *uint256.Int) []*uint256.Int {
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
