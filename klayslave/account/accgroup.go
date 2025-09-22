package account

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"time"
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

func (a *AccGroup) CreateAccountsPerAccGrp(nUserForSignedTx int, nUserForUnsignedTx int, nUserForNewAccounts int, nUserForGaslessRevertTx int, nUserForGaslessApproveTx int, tcStrList []string, gEndpoint string) {
	// Try to load existing accounts from most recent accounts json file
	files, err := filepath.Glob("accounts-*.json")
	if err != nil {
		log.Printf("Failed to glob account files: %v", err)
	} else if len(files) > 0 {
		// Sort files by name descending to get most recent
		sort.Sort(sort.Reverse(sort.StringSlice(files)))

		f, err := os.Open(files[0])
		if err != nil {
			log.Printf("Failed to open account file %s: %v", files[0], err)
		} else {
			defer f.Close()

			var accounts [][]*Account
			if err := json.NewDecoder(f).Decode(&accounts); err != nil {
				log.Printf("Failed to decode accounts from %s: %v", files[0], err)
			} else {
				// Reuse existing accounts
				a.accLists = accounts
				log.Printf("Loaded existing accounts from %s", files[0])
				return
			}
		}
	}

	for idx, nUser := range []int{nUserForSignedTx, nUserForUnsignedTx, nUserForNewAccounts, nUserForGaslessRevertTx, nUserForGaslessApproveTx} {
		println(idx, " Account Group Preparation...")
		for i := 0; i < nUser; i++ {
			account := NewAccount(i)
			a.AddAccToListByName(account, AccList(idx))
			fmt.Printf("%v\n", account.address.String())
		}
	}

	fn := fmt.Sprintf("accounts-%s.json", time.Now().Format("20060102_150405"))
	f, err := os.OpenFile(fn, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(a.accLists)

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
