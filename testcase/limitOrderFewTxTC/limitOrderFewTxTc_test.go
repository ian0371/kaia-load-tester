package limitOrderFewTxTC

import (
	"bytes"
	"encoding/json"
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/core/orderbook"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
	"github.com/kaiachain/kaia-load-tester/klayslave/account"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestGenOrderTx(t *testing.T) {
	var (
		from     = account.NewAccount(0)
		side     = orderbook.BUY
		price    = big.NewInt(2e18)
		quantity = big.NewInt(1e18)
	)

	tx, err := from.GenNewOrderTx(baseToken, quoteToken, side, price, quantity, orderType)
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.True(t, len(tx.Data()) > 0)
	// assert.NotNil(t, tx.GetOrderContext())

	data, err := tx.MarshalBinary()
	require.NoError(t, err)
	// hexData := hexutil.Encode(data)
	var decodedTx types.Transaction
	rlp.Decode(bytes.NewReader(data), &decodedTx)

	assert.True(t, len(decodedTx.Data()) > 0)
	jsonData, err := json.Marshal(string(decodedTx.Data()[1:]))
	require.NoError(t, err)
	t.Log(string(jsonData))
}
