package account

import (
	"math"
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestHierarchicalDistributeTransfer(t *testing.T) {
	var (
		rich       = NewAccount(0)
		accs       = make([]*Account, 0)
		balanceMap = make(map[common.Address]uint64)
		value      = big.NewInt(10000)
	)

	balanceMap[rich.GetAddress()] = math.MaxUint64
	mu := sync.Mutex{}
	for i := 0; i < 10000; i++ {
		accs = append(accs, NewAccount(0))
	}

	t.Logf("rich: %s, balance: %d", rich.GetAddress().Hex(), balanceMap[rich.GetAddress()])
	HierarchicalDistribute(accs, rich, value, big.NewInt(0), func(from, to *Account, value *big.Int) {
		if from == nil || to == nil || value == nil {
			t.Fatalf("all must not be nil, from: %v, to: %v, value: %v", from, to, value)
		}
		t.Logf("from: %s, to: %s, value: %d", from.GetAddress().Hex(), to.GetAddress().Hex(), value.Uint64())
		mu.Lock()
		if balanceMap[from.GetAddress()] < value.Uint64() {
			t.Fatalf("balance of %s is %d, but need to transfer %d", from.GetAddress().Hex(), balanceMap[from.GetAddress()], value.Uint64())
		}
		balanceMap[from.GetAddress()] = balanceMap[from.GetAddress()] - value.Uint64()
		balanceMap[to.GetAddress()] = balanceMap[to.GetAddress()] + value.Uint64()
		mu.Unlock()
	})

	for i, acc := range accs {
		assert.Equal(t, value.Uint64(), balanceMap[acc.GetAddress()], "balance of account[%d] is %d", i, balanceMap[acc.GetAddress()])
	}
}
