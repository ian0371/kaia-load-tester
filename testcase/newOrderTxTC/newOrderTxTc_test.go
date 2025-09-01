package newOrderTxTC

import (
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/core/orderbook"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
)

func TestNewOrderTxTC(t *testing.T) {
	mr := orderbook.NewMarketRules()
	for range 1000 {
		var (
			base           = uint256.NewInt(uint64(1e18)) // 0.3-0.7
			priceOffset    = uint256.NewInt(uint64(rand.Intn(5) + 3))
			quantityOffset = uint256.NewInt(uint64(rand.Intn(5) + 3))
			price          = new(uint256.Int).Mul(base, priceOffset) // 3-7
			quantity       = new(uint256.Int).Mul(base, quantityOffset)
		)

		assert.NoError(t, mr.ValidateOrderPrice(price), "price: %s", price.String())
		assert.NoError(t, mr.ValidateOrderQuantity(price, quantity), "price: %s, quantity: %s", price.String(), quantity.String())
		assert.NoError(t, mr.ValidateMinimumOrderValue(price, quantity), "price: %s, quantity: %s", price.String(), quantity.String())
	}
}
