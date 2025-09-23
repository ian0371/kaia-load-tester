package account

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// AccList defines the enum for accList
type AccList int

const (
	AccListForSignedTx AccList = iota
	AccListForUnsignedTx
	AccListForNewAccounts
	AccListForGaslessRevertTx
	AccListForGaslessApproveTx
	AccListEnd
)

// TestContract defines the enum for TestContract
type TestContract int

const (
	ContractErc20 TestContract = iota
	ContractErc721
	ContractStorageTrie
	ContractGeneral
	ContractGaslessToken
	ContractGaslessSwapRouter
	ContractEnd
)

type AccLoader func(*AccGroup)

type AccGroup struct {
	containsUnsignedAccGrp bool

	accLists  [][]*Account
	contracts []*Account
}

func NewAccGroup(chainId *big.Int, gasPrice *big.Int, baseFee *big.Int, batchSize int, contains bool) *AccGroup {
	SetChainID(chainId)
	SetGasPrice(gasPrice)
	SetBaseFee(baseFee)
	SetBatchSize(batchSize)

	return &AccGroup{
		containsUnsignedAccGrp: contains,
		accLists:               make([][]*Account, AccListEnd),
		contracts:              make([]*Account, ContractEnd),
	}
}
func (a *AccGroup) Load(loader AccLoader) { loader(a) }

func (a *AccGroup) GetTestContractByName(t TestContract) *Account { return a.contracts[t] }
func (a *AccGroup) GetAccListByName(t AccList) []*Account         { return a.accLists[t] }

func (a *AccGroup) SetTestContractByName(c *Account, t TestContract) { a.contracts[t] = c }
func (a *AccGroup) SetAccListByName(accs []*Account, t AccList) {
	for _, acc := range accs {
		a.accLists[t] = append(a.accLists[t], acc)
	}
}

func (a *AccGroup) AddAccToListByName(acc *Account, t AccList) {
	a.accLists[t] = append(a.accLists[t], acc)
}

// loadAccountsFromFile attempts to load accounts from the most recent accounts-*.json file
// Returns true if accounts were successfully loaded, false otherwise
func loadAccountsFromFile() (*AccGroup, error) {
	files, err := filepath.Glob("accounts-*.json")
	if err != nil {
		return nil, fmt.Errorf("failed to glob account files: %v", err)
	}

	if len(files) == 0 {
		return nil, nil // No files found, not an error
	}

	// Sort files by name descending to get most recent
	sort.Sort(sort.Reverse(sort.StringSlice(files)))

	return loadAccountsFromSpecificFile(files[0])
}

func loadAccountsFromSpecificFile(filename string) (*AccGroup, error) {
	accGroup := &AccGroup{
		accLists:  make([][]*Account, AccListEnd),
		contracts: make([]*Account, ContractEnd),
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open account file %s: %v", filename, err)
	}
	defer f.Close()

	var accounts AccGroup
	if err := json.NewDecoder(f).Decode(&accounts); err != nil {
		return nil, fmt.Errorf("failed to decode accounts from %s: %v", filename, err)
	}

	accGroup.containsUnsignedAccGrp = accounts.containsUnsignedAccGrp
	accGroup.accLists = accounts.accLists
	accGroup.contracts = accounts.contracts

	return accGroup, nil
}

// saveAccountsToFile saves the current accounts to a timestamped JSON file
func (a *AccGroup) saveAccountsToFile() error {
	filename := fmt.Sprintf("accounts-%s.json", time.Now().Format("20060102_150405"))
	return a.saveAccountsToSpecificFile(filename)
}

// saveAccountsToSpecificFile saves the current accounts to a specific file
func (a *AccGroup) saveAccountsToSpecificFile(filename string) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", filename, err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(a); err != nil {
		return fmt.Errorf("failed to encode accounts to %s: %v", filename, err)
	}

	log.Printf("Saved accounts to %s", filename)
	return nil
}

// getTotalAccountCount returns the total number of accounts across all lists
func (a *AccGroup) getTotalAccountCount() int {
	total := 0
	for _, accList := range a.accLists {
		total += len(accList)
	}
	return total
}

func (a *AccGroup) CreateAccountsPerAccGrp(nUserForSignedTx int, nUserForUnsignedTx int, nUserForNewAccounts int, nUserForGaslessRevertTx int, nUserForGaslessApproveTx int, tcStrList []string, gEndpoint string) {
	// Try to load existing accounts from file
	loaded, err := loadAccountsFromFile()
	if err != nil {
		log.Printf("Error loading accounts from file: %v", err)
	}

	// Check if we have enough accounts loaded
	totalNeeded := nUserForSignedTx + nUserForUnsignedTx + nUserForNewAccounts + nUserForGaslessRevertTx + nUserForGaslessApproveTx
	if loaded != nil && loaded.getTotalAccountCount() >= totalNeeded {
		a.containsUnsignedAccGrp = loaded.containsUnsignedAccGrp
		a.accLists = loaded.accLists
		a.contracts = loaded.contracts
		return // We have enough accounts loaded from file
	}

	// Create new accounts if not loaded or not enough
	for idx, nUser := range []int{nUserForSignedTx, nUserForUnsignedTx, nUserForNewAccounts, nUserForGaslessRevertTx, nUserForGaslessApproveTx} {
		println(idx, " Account Group Preparation...")
		for i := 0; i < nUser; i++ {
			account := NewAccount(i)
			a.AddAccToListByName(account, AccList(idx))
		}
	}

	// Save the newly created accounts
	if err := a.saveAccountsToFile(); err != nil {
		log.Fatalf("Failed to save accounts: %v", err)
	}

	// Unlock AccGrpForUnsignedTx if needed
	for _, tcName := range tcStrList {
		if tcName != "transferUnsignedTx" {
			continue
		}
		// If at least one task needs unlocking, unlock the accGrp for unsignedTx.
		for _, acc := range a.GetAccListByName(AccListForUnsignedTx) {
			acc.ImportUnLockAccount(gEndpoint)
		}
		break
	}
}

func (a *AccGroup) SetAccGrpByActivePercent(activeUserPercent int) {
	for i, accGrp := range a.accLists {
		numActiveAccGrpForSignedTx := len(accGrp) * activeUserPercent / 100
		if numActiveAccGrpForSignedTx == 0 {
			a.accLists[i] = nil
			continue
		}

		a.accLists[i] = accGrp[:numActiveAccGrpForSignedTx]
	}
}

func (a *AccGroup) GetValidAccGrp() []*Account {
	var accGrp []*Account
	for _, acc := range a.GetAccListByName(AccListForSignedTx) {
		accGrp = append(accGrp, acc)
	}
	// if !a.cfg.InTheTcList("transferUnsignedTx") {
	if !a.containsUnsignedAccGrp {
		return accGrp
	}
	for _, acc := range a.GetAccListByName(AccListForUnsignedTx) {
		accGrp = append(accGrp, acc)
	}
	return accGrp
}

type AccountJSON struct {
	Address    string `json:"address"`
	PrivateKey string `json:"privateKey"`
	Nonce      uint64 `json:"nonce"`
	TimeNonce  uint64 `json:"timeNonce"`
}
type AccGroupJSON struct {
	ContainsUnsignedAccGrp bool `json:"containsUnsignedAccGrp"`
	Accounts               [][]AccountJSON
	Contracts              []AccountJSON
}

func (a AccGroup) MarshalJSON() ([]byte, error) {
	ret := AccGroupJSON{
		ContainsUnsignedAccGrp: a.containsUnsignedAccGrp,
		Accounts:               make([][]AccountJSON, len(a.accLists)),
		Contracts:              make([]AccountJSON, len(a.contracts)),
	}
	for i, accList := range a.accLists {
		ret.Accounts[i] = make([]AccountJSON, len(accList))
		for j, acc := range accList {
			// Ensure private key is always 64 hex characters (32 bytes)
			privateKeyBytes := acc.privateKey.D.Bytes()
			if len(privateKeyBytes) < 32 {
				// Pad with leading zeros if needed
				paddedKey := make([]byte, 32)
				copy(paddedKey[32-len(privateKeyBytes):], privateKeyBytes)
				privateKeyBytes = paddedKey
			}
			ret.Accounts[i][j] = AccountJSON{
				Address:    acc.address.String(),
				PrivateKey: hexutil.Encode(privateKeyBytes),
				Nonce:      acc.nonce,
				TimeNonce:  acc.timenonce,
			}
		}
	}

	// Handle contracts
	for i, contract := range a.contracts {
		if contract != nil {
			// Ensure private key is always 64 hex characters (32 bytes)
			privateKeyBytes := contract.privateKey.D.Bytes()
			if len(privateKeyBytes) < 32 {
				// Pad with leading zeros if needed
				paddedKey := make([]byte, 32)
				copy(paddedKey[32-len(privateKeyBytes):], privateKeyBytes)
				privateKeyBytes = paddedKey
			}
			ret.Contracts[i] = AccountJSON{
				Address:    contract.address.String(),
				PrivateKey: hexutil.Encode(privateKeyBytes),
				Nonce:      contract.nonce,
				TimeNonce:  contract.timenonce,
			}
		}
	}

	return json.Marshal(ret)
}

func (a *AccGroup) UnmarshalJSON(data []byte) error {
	var src AccGroupJSON
	if err := json.Unmarshal(data, &src); err != nil {
		return err
	}

	a.containsUnsignedAccGrp = src.ContainsUnsignedAccGrp
	a.contracts = make([]*Account, len(src.Contracts))
	for i, acc := range src.Contracts {
		// Skip empty contracts (nil entries)
		if acc.PrivateKey == "" || acc.Address == "" {
			a.contracts[i] = nil
			continue
		}

		// lstrip "0x" from acc.PrivateKey
		acc.PrivateKey = strings.TrimPrefix(acc.PrivateKey, "0x")
		pk, err := crypto.HexToECDSA(acc.PrivateKey)
		if err != nil {
			return err
		}
		a.contracts[i] = &Account{
			address:    common.HexToAddress(acc.Address),
			privateKey: pk,
		}
	}

	a.accLists = make([][]*Account, len(src.Accounts))
	for i := range src.Accounts {
		a.accLists[i] = make([]*Account, len(src.Accounts[i]))
		for j, acc := range src.Accounts[i] {
			acc.PrivateKey = strings.TrimPrefix(acc.PrivateKey, "0x")
			pk, err := crypto.HexToECDSA(acc.PrivateKey)
			if err != nil {
				return err
			}
			a.accLists[i][j] = &Account{
				address:    common.HexToAddress(acc.Address),
				privateKey: pk,
				nonce:      acc.Nonce,
				timenonce:  acc.TimeNonce,
			}
		}
	}
	return nil
}
