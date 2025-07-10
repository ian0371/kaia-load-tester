package account

import (
	"fmt"
	"log"
	"math/big"

	"github.com/kaiachain/kaia-load-tester/klayslave/client"
	"github.com/kaiachain/kaia/common"
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

func NewAccGroup(chainId *big.Int, gasPrice *big.Int, baseFee *big.Int, contains bool) *AccGroup {
	SetChainID(chainId)
	SetGasPrice(gasPrice)
	SetBaseFee(baseFee)

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
	for idx, nUser := range []int{nUserForSignedTx, nUserForUnsignedTx, nUserForNewAccounts, nUserForGaslessRevertTx, nUserForGaslessApproveTx} {
		println(idx, " Account Group Preparation...")
		for i := 0; i < nUser; i++ {
			account := NewAccount(i)
			a.AddAccToListByName(account, AccList(idx))
			fmt.Printf("%v\n", account.address.String())
		}
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
	//if !a.cfg.InTheTcList("transferUnsignedTx") {
	if !a.containsUnsignedAccGrp {
		return accGrp
	}
	for _, acc := range a.GetAccListByName(AccListForUnsignedTx) {
		accGrp = append(accGrp, acc)
	}
	return accGrp
}

func (a *AccGroup) DeployTestContracts(tcList []string, globalReservoir, localReservoir *Account, gCli *client.Client, chargeValue *big.Int, skipDeploys map[TestContract]common.Address) {
	inTheTcList := func(testNames []string) bool {
		for _, tcName := range tcList {
			for _, target := range testNames {
				if tcName == target {
					return true
				}
			}
		}
		return false
	}

	for idx, info := range TestContractInfos {
		testContractType := TestContract(idx)
		if testContractType != ContractGeneral && !inTheTcList(info.testNames) {
			continue
		}

		if address, skip := skipDeploys[testContractType]; skip {
			a.contracts[idx] = NewKaiaAccountWithAddr(0, address)
		} else {
			localReservoir.TransferSignedTxWithGuaranteeRetry(gCli, info.deployer, chargeValue)
			a.contracts[idx] = info.deployer.SmartContractDeployWithGuaranteeRetry(gCli, info.Bytecode, info.contractName)
		}

		// additional work - erc20 token charging or erc721 minting
		if TestContract(idx) == ContractErc20 {
			log.Printf("Start erc20 token charging to the test account group")
			TestContractInfos[ContractErc20].deployer.SmartContractExecutionWithGuaranteeRetry(gCli, a.contracts[ContractErc20], TestContractInfos[ContractErc20].GenData(localReservoir.address, big.NewInt(1e11)))
			ConcurrentTransactionSend(a.GetValidAccGrp(), func(acc *Account) {
				localReservoir.SmartContractExecutionWithGuaranteeRetry(gCli, a.contracts[ContractErc20], TestContractInfos[ContractErc20].GenData(acc.address, big.NewInt(1e4)))
			})
		} else if TestContract(idx) == ContractErc721 {
			log.Printf("Start erc721 nft minting to the test account group(similar to erc20 token charging)")
			localReservoir.MintERC721ToTestAccounts(gCli, a.GetValidAccGrp(), a.GetTestContractByName(ContractErc721).GetAddress(), 5)
		} else if TestContract(idx) == ContractGaslessToken && inTheTcList([]string{"gaslessTransactionTC", "gaslessOnlyApproveTC"}) {
			log.Printf("Start gasless test token charging to the test account group")
			lenValidAccGrp := big.NewInt(int64(len(a.GetValidAccGrp())))
			lenGaslessApproveAccGrp := big.NewInt(int64(len(a.GetAccListByName(AccListForGaslessApproveTx))))
			totalChargeValue := new(big.Int).Mul(chargeValue, new(big.Int).Add(lenValidAccGrp, lenGaslessApproveAccGrp))
			// ContractGaslessToken's GenData generate data of approve. So can use ERC20's genData for transfer.
			globalReservoir.SmartContractExecutionWithGuaranteeRetry(gCli, a.contracts[ContractGaslessToken], TestContractInfos[ContractErc20].GenData(localReservoir.address, totalChargeValue))

			// accounts(validAccGrp + gaslessApproveAccGrp) should be charged.
			accounts := a.GetValidAccGrp()
			accounts = append(accounts, a.GetAccListByName(AccListForGaslessApproveTx)...)
			ConcurrentTransactionSend(accounts, func(acc *Account) {
				localReservoir.SmartContractExecutionWithGuaranteeRetry(gCli, a.contracts[ContractGaslessToken], TestContractInfos[ContractErc20].GenData(acc.address, chargeValue))
			})
		}
	}
}
