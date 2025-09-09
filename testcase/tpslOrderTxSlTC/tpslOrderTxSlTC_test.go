package tpslOrderTxSlTC

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/orderbook"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/kaiachain/kaia-load-tester/klayslave/account"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validate extracted from OrderContext.validate (core/types/tx_input.go)
func validate(t *types.OrderContext, marketRules *orderbook.MarketRules) error {
	priceMax := new(big.Int).Sub(
		new(big.Int).Exp(big.NewInt(10), big.NewInt(29), nil),
		big.NewInt(1),
	)
	quantityMax := new(big.Int).Sub(
		new(big.Int).Exp(big.NewInt(10), big.NewInt(41), nil),
		big.NewInt(1),
	)
	const MaxTokenLength = 20
	if t.L1Owner == (common.Address{}) {
		return errors.New("from address is zero")
	}
	if t.BaseToken == "" || t.QuoteToken == "" {
		return errors.New("base token or quote token is empty")
	}
	if len(t.BaseToken) > MaxTokenLength || len(t.QuoteToken) > MaxTokenLength {
		return errors.New("base token or quote token is too long")
	}
	if t.Side != 0 && t.Side != 1 {
		return errors.New("invalid side: must be 0 (buy) or 1 (sell)")
	}
	if t.OrderType != types.LIMIT && t.OrderType != types.MARKET {
		return errors.New("invalid order type: must be 0 (limit) or 1 (market)")
	}
	if t.OrderType == types.LIMIT && (t.Price == nil || t.Price.Sign() <= 0) {
		return errors.New("price must be positive for limit orders")
	}
	if t.OrderType == types.MARKET && t.Price == nil {
		t.Price = big.NewInt(0) // For market orders, price is not used but must be non-nil
	}
	if t.OrderType == types.MARKET && t.TPSL != nil {
		return errors.New("TPSL orders cannot be market orders")
	}
	if t.Quantity == nil || t.Quantity.Sign() <= 0 {
		return errors.New("quantity must be positive")
	}
	if t.OrderType != 0 && t.OrderType != 1 {
		return errors.New("invalid order type: must be 0 (limit) or 1 (market)")
	}
	if t.Price != nil && t.Price.Cmp(priceMax) > 0 {
		return errors.New("price exceeds price max value")
	}
	if t.Quantity != nil && t.Quantity.Cmp(quantityMax) > 0 {
		return errors.New("quantity exceeds quantity max value")
	}
	if t.TPSL != nil {
		if t.TPSL.TPLimit == nil || t.TPSL.SLTrigger == nil {
			return errors.New("TPSL TPLimit and SLTrigger must be set")
		}
		if t.TPSL.TPLimit != nil && t.TPSL.TPLimit.Cmp(priceMax) > 0 {
			return errors.New("TPSL TPLimit exceeds price max value")
		}
		if t.TPSL.SLTrigger != nil && t.TPSL.SLTrigger.Cmp(priceMax) > 0 {
			return errors.New("TPSL SLTrigger exceeds price max value")
		}
		if t.TPSL.SLLimit != nil && t.TPSL.SLLimit.Cmp(priceMax) > 0 {
			return errors.New("TPSL SLLimit exceeds price max value")
		}
		if t.TPSL.TPLimit.Sign() <= 0 || t.TPSL.SLTrigger.Sign() <= 0 {
			return errors.New("TPSL TPLimit and SLTrigger must be positive")
		}
		if t.Side == types.BUY && t.TPSL.TPLimit.Cmp(t.Price) <= 0 {
			return fmt.Errorf("TPSL TPLimit %s must be greater than order price %s", t.TPSL.TPLimit, t.Price)
		}
		if t.Side == types.SELL && t.TPSL.TPLimit.Cmp(t.Price) >= 0 {
			return fmt.Errorf("TPSL TPLimit %s must be less than order price %s", t.TPSL.TPLimit, t.Price)
		}
		if t.Side == types.BUY && t.TPSL.SLTrigger.Cmp(t.Price) >= 0 {
			return fmt.Errorf("TPSL SLTrigger %s must be less than order price %s", t.TPSL.SLTrigger, t.Price)
		}
		if t.Side == types.SELL && t.TPSL.SLTrigger.Cmp(t.Price) <= 0 {
			return fmt.Errorf("TPSL SLTrigger %s must be greater than order price %s", t.TPSL.SLTrigger, t.Price)
		}
	}

	if t.OrderType == types.MARKET {
		// Market order validation using best price
		// Note: we cannot use orderbook.BUY/SELL here due to parameter name conflict
		// So we define the side based on t.Side
		panic("not implemented")
	} else if t.OrderType == types.LIMIT {
		// Price tick size validation
		priceUint := uint256.MustFromBig(t.Price)
		if err := marketRules.ValidateOrderPrice(priceUint); err != nil {
			return fmt.Errorf("price validation failed: %v", err)
		}

		// Quantity lot size validation (convert quote mode to base for validation)
		qty := t.Quantity
		if t.OrderMode == 1 { // QUOTE_MODE
			qty = common.BigIntDivScaledDecimal(t.Quantity, t.Price)
		}
		qtyUint := uint256.MustFromBig(qty)

		if err := marketRules.ValidateOrderQuantity(priceUint, qtyUint); err != nil {
			return fmt.Errorf("quantity validation failed: %v", err)
		}

		// Minimum order value validation ($1)
		if err := marketRules.ValidateMinimumOrderValue(priceUint, qtyUint); err != nil {
			return err
		}

		// TPSL price validation
		if t.TPSL != nil {
			// TP limit price validation
			if t.TPSL.TPLimit != nil {
				tpPriceUint := uint256.MustFromBig(t.TPSL.TPLimit)
				if err := marketRules.ValidateOrderPrice(tpPriceUint); err != nil {
					return fmt.Errorf("TPSL TP limit price validation failed: %v", err)
				}
			}

			// SL trigger price validation
			if t.TPSL.SLTrigger != nil {
				slTriggerUint := uint256.MustFromBig(t.TPSL.SLTrigger)
				if err := marketRules.ValidateOrderPrice(slTriggerUint); err != nil {
					return fmt.Errorf("TPSL SL trigger price validation failed: %v", err)
				}
			}

			// SL limit price validation (if exists)
			if t.TPSL.SLLimit != nil {
				slLimitUint := uint256.MustFromBig(t.TPSL.SLLimit)
				if err := marketRules.ValidateOrderPrice(slLimitUint); err != nil {
					return fmt.Errorf("TPSL SL limit price validation failed: %v", err)
				}
			}
		}
	}
	return nil
}

func TestNewOrderCtxWithTpsl(t *testing.T) {
	var (
		from      = account.NewAccount(0)
		side      = orderbook.BUY
		price     = scaleUp(3)
		quantity  = scaleUp(1)
		tpLimit   = scaleUp(9999)
		slTrigger = scaleUp(2)
		slLimit   = scaleUp(1)
		orderType = orderbook.LIMIT
	)

	ctx := from.NewOrderCtxWithTpsl(baseToken, quoteToken, side, price, quantity, orderType, tpLimit, slTrigger, slLimit)
	input, err := types.WrapTxAsInput(ctx)
	require.NoError(t, err)
	tx := types.NewTransaction(
		0,
		types.DexAddress,
		common.Big0,
		0,
		common.Big0,
		input,
	)
	require.NoError(t, err)
	require.NotNil(t, tx.GetOrderContext())
	assert.NoError(t, validate(tx.GetOrderContext(), orderbook.NewMarketRules()))
}
