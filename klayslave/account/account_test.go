package account

import (
	"encoding/json"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestMarshalJson(t *testing.T) {
	var (
		accs = AccGroup{
			accLists: make([][]*Account, 0),
		}
		newAccs AccGroup
	)

	accs.accLists = append(accs.accLists, []*Account{NewAccount(0), NewAccount(1)})
	accs.accLists = append(accs.accLists, []*Account{NewAccount(2)})
	accs.accLists = append(accs.accLists, []*Account{NewAccount(3), NewAccount(4), NewAccount(5)})

	jsonData, err := json.Marshal(accs)
	if err != nil {
		t.Fatalf("Failed to marshal accounts: %v", err)
	}
	t.Log(string(jsonData))

	err = json.Unmarshal(jsonData, &newAccs)
	if err != nil {
		t.Fatalf("Failed to unmarshal accounts: %v", err)
	}
	assert.Equal(t, 3, len(newAccs.accLists))
	assert.Equal(t, 2, len(newAccs.accLists[0]))
	assert.Equal(t, 1, len(newAccs.accLists[1]))
	assert.Equal(t, 3, len(newAccs.accLists[2]))

	for i, accList := range newAccs.accLists {
		for j, acc := range accList {
			assert.Equal(t, acc.GetAddress(), accs.accLists[i][j].GetAddress())
			assert.Equal(t, acc.PrivateKey(), accs.accLists[i][j].PrivateKey())
		}
	}
}

func Test_SaveLoad(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "account_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test AccGroup with some accounts
	accGroup := &AccGroup{
		containsUnsignedAccGrp: true,
		accLists:               make([][]*Account, AccListEnd),
		contracts:              make([]*Account, ContractEnd),
	}

	// Add some test accounts
	accGroup.accLists[AccListForSignedTx] = []*Account{NewAccount(0), NewAccount(1)}
	accGroup.accLists[AccListForUnsignedTx] = []*Account{NewAccount(2)}

	// Test saving to a specific file
	filename := filepath.Join(tempDir, "test_accounts.json")
	err = accGroup.saveAccountsToSpecificFile(filename)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(filename)
	require.NoError(t, err)

	// Verify file content by loading it back
	loaded, err := loadAccountsFromSpecificFile(filename)
	require.NoError(t, err)
	assert.NotNil(t, loaded)

	// Verify the loaded data matches original
	assert.Equal(t, accGroup.containsUnsignedAccGrp, loaded.containsUnsignedAccGrp)
	assert.Equal(t, len(accGroup.accLists[AccListForSignedTx]), len(loaded.accLists[AccListForSignedTx]))
	assert.Equal(t, len(accGroup.accLists[AccListForUnsignedTx]), len(loaded.accLists[AccListForUnsignedTx]))
}

func TestLoadAccountsFromFile_NoFiles(t *testing.T) {
	// Test loading when no files exist
	loaded, err := loadAccountsFromSpecificFile("empty_file")
	assert.Error(t, err)
	assert.Nil(t, loaded)
}
