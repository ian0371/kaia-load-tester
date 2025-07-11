package main

//go:generate abigen --sol cpuHeavyTC/CPUHeavy.sol --pkg cpuHeavyTC --out cpuHeavyTC/CPUHeavy.go
//go:generate abigen --sol userStorageTC/UserStorage.sol --pkg userStorageTC --out userStorageTC/UserStorage.go

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"runtime"

	"github.com/kaiachain/kaia-load-tester/klayslave/account"
	"github.com/kaiachain/kaia-load-tester/klayslave/config"
	"github.com/kaiachain/kaia-load-tester/testcase"
	"github.com/kaiachain/kaia-load-tester/testcase/erc20TransferTC"
	"github.com/kaiachain/kaia-load-tester/testcase/erc721TransferTC"
	"github.com/kaiachain/kaia-load-tester/testcase/ethereumTxAccessListTC"
	"github.com/kaiachain/kaia-load-tester/testcase/ethereumTxDynamicFeeTC"
	"github.com/kaiachain/kaia-load-tester/testcase/ethereumTxLegacyTC"
	"github.com/kaiachain/kaia-load-tester/testcase/gaslessOnlyApproveTC"
	"github.com/kaiachain/kaia-load-tester/testcase/gaslessRevertTransactionTC"
	"github.com/kaiachain/kaia-load-tester/testcase/gaslessTransactionTC"
	"github.com/kaiachain/kaia-load-tester/testcase/newEthereumAccessListTC"
	"github.com/kaiachain/kaia-load-tester/testcase/newEthereumDynamicFeeTC"
	"github.com/kaiachain/kaia-load-tester/testcase/newFeeDelegatedSmartContractExecutionTC"
	"github.com/kaiachain/kaia-load-tester/testcase/newFeeDelegatedSmartContractExecutionWithRatioTC"
	"github.com/kaiachain/kaia-load-tester/testcase/newSmartContractExecutionTC"
	"github.com/kaiachain/kaia-load-tester/testcase/storageTrieWriteTC"
	"github.com/kaiachain/kaia/accounts/abi/bind"
	"github.com/kaiachain/kaia/api/debug"
	"github.com/kaiachain/kaia/console"
	"github.com/myzhan/boomer"
	"github.com/urfave/cli"
)

var app = cli.NewApp()

func init() {
	app.Name = filepath.Base(os.Args[0])
	app.Usage = "This is for kaia load testing."
	app.Version = config.GetVersionWithCommit() // To see the version, run 'klayslave -v'
	app.HideVersion = false
	app.Copyright = "Copyright 2024 Kaia-load-tester authors"
	app.Flags = append(config.Flags, config.BoomerFlags...)

	// This app doesn't provide any subcommand
	//		app.Commands = []*cli.Command{}
	//		sort.Sort(cli.CommandsByName(app.Commands))
	//		app.CommandNotFound = nodecmd.CommandNotExist
	// app.OnUsageError = nodecmd.OnUsageError
	app.Before = func(cli *cli.Context) error {
		//runtime.GOMAXPROCS(runtime.NumCPU())
		if runtime.GOOS == "darwin" {
			return nil
		}
		return config.SetRLimit()
	}
	app.Action = RunAction
	app.After = func(cli *cli.Context) error {
		debug.Exit()
		console.Stdin.Close() // Resets terminal mode.
		return nil
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func RunAction(ctx *cli.Context) {
	cfg := config.NewConfig(ctx)
	accGrp := account.NewAccGroup(cfg.GetChainID(), cfg.GetGasPrice(), cfg.GetBaseFee(), cfg.InTheTcList("transferUnsignedTx"))
	var nUserForGaslessRevertTx, nUserForGaslessApproveTx int = 0, 0
	if cfg.InTheTcList("gaslessRevertTransactionTC") {
		nUserForGaslessRevertTx = cfg.GetNUserForSigned() // same as nUserForSignedTx
	}
	if cfg.InTheTcList("gaslessOnlyApproveTC") {
		nUserForGaslessApproveTx = cfg.GetNUserForSigned() // same as nUserForSignedTx
	}
	accGrp.CreateAccountsPerAccGrp(cfg.GetNUserForSigned(), cfg.GetNUserForUnsigned(), cfg.GetNUserForNewAccounts(), nUserForGaslessRevertTx, nUserForGaslessApproveTx, cfg.GetTcStrList(), cfg.GetGEndpoint())

	createTestAccGroupsAndPrepareContracts(cfg, accGrp)
	tasks := cfg.GetExtendedTasks()
	initializeTasks(cfg, accGrp, tasks)
	boomer.Run(toBoomerTasks(tasks)...)
}

// TODO-kaia-load-tester: remove global variables in the tc packages
func setSmartContractAddressPerPackage(a *account.AccGroup) {
	erc20TransferTC.SmartContractAccount = a.GetTestContractByName(account.ContractErc20)
	erc721TransferTC.SmartContractAccount = a.GetTestContractByName(account.ContractErc721)
	storageTrieWriteTC.SmartContractAccount = a.GetTestContractByName(account.ContractStorageTrie)

	newSmartContractExecutionTC.SmartContractAccount = a.GetTestContractByName(account.ContractGeneral)
	newFeeDelegatedSmartContractExecutionTC.SmartContractAccount = a.GetTestContractByName(account.ContractGeneral)
	newFeeDelegatedSmartContractExecutionWithRatioTC.SmartContractAccount = a.GetTestContractByName(account.ContractGeneral)
	ethereumTxLegacyTC.SmartContractAccount = a.GetTestContractByName(account.ContractGeneral)
	ethereumTxAccessListTC.SmartContractAccount = a.GetTestContractByName(account.ContractGeneral)
	ethereumTxDynamicFeeTC.SmartContractAccount = a.GetTestContractByName(account.ContractGeneral)
	newEthereumAccessListTC.SmartContractAccount = a.GetTestContractByName(account.ContractGeneral)
	newEthereumDynamicFeeTC.SmartContractAccount = a.GetTestContractByName(account.ContractGeneral)

	gaslessTransactionTC.TestTokenAccount = a.GetTestContractByName(account.ContractGaslessToken)
	gaslessTransactionTC.GsrAccount = a.GetTestContractByName(account.ContractGaslessSwapRouter)
	gaslessRevertTransactionTC.TestTokenAccount = a.GetTestContractByName(account.ContractGaslessToken)
	gaslessRevertTransactionTC.GsrAccount = a.GetTestContractByName(account.ContractGaslessSwapRouter)
	gaslessOnlyApproveTC.TestTokenAccount = a.GetTestContractByName(account.ContractGaslessToken)
	gaslessOnlyApproveTC.GsrAccount = a.GetTestContractByName(account.ContractGaslessSwapRouter)
}

// createTestAccGroupsAndPrepareContracts do every init steps before task.Init
// those steps are about deploying test contracts and
func createTestAccGroupsAndPrepareContracts(cfg *config.Config, accGrp *account.AccGroup) *account.Account {
	if len(cfg.GetChargeValue().Bits()) == 0 {
		return nil
	}

	// 1. Import global reservoir Account and create local reservoir account
	globalReservoirAccount := account.GetAccountFromKey(0, cfg.GetRichWalletPrivateKey())
	localReservoirAccount := account.NewAccount(0)

	// 2. charge local reservoir
	_ = globalReservoirAccount.GetNonce(cfg.GetGCli())
	revertGroupChargeValue := new(big.Int).Mul(cfg.GetChargeValue(), big.NewInt(int64(len(accGrp.GetAccListByName(account.AccListForGaslessRevertTx)))))
	approveGroupChargeValue := new(big.Int).Mul(cfg.GetChargeValue(), big.NewInt(int64(len(accGrp.GetAccListByName(account.AccListForGaslessApproveTx)))))
	initialLiquidity := account.GetInitialLiquidity()
	totalChargeValue := new(big.Int).Add(cfg.GetTotalChargeValue(), new(big.Int).Add(initialLiquidity, new(big.Int).Add(revertGroupChargeValue, approveGroupChargeValue)))
	tx := globalReservoirAccount.TransferSignedTxWithGuaranteeRetry(cfg.GetGCli(), localReservoirAccount, totalChargeValue)
	receipt, err := bind.WaitMined(context.Background(), cfg.GetGCli(), tx)
	if err != nil {
		log.Fatalf("receipt failed, err:%v", err.Error())
	}
	if receipt.Status != 1 {
		log.Fatalf("transfer for reservoir failed, localReservoir")
	}

	// 3. charge KAIA
	log.Printf("Start charging KLAY to test accounts")
	accs := accGrp.GetValidAccGrp()
	accs = append(accs, accGrp.GetAccListByName(account.AccListForGaslessRevertTx)...)  // for avoid validation
	accs = append(accs, accGrp.GetAccListByName(account.AccListForGaslessApproveTx)...) // for avoid validation
	account.ConcurrentTransactionSend(accs, cfg.GetChargeParallelNum(), func(acc *account.Account) {
		localReservoirAccount.TransferSignedTxWithGuaranteeRetry(cfg.GetGCli(), acc, cfg.GetChargeValue())
	})
	log.Printf("Finished charging KLAY to %d test account(s)\n", len(accs))

	// Wait, charge KAIA happen in 100% of all created test accounts
	// But, from here including prepareTestContracts like MintERC721, only 20% of account happens
	accGrp.SetAccGrpByActivePercent(cfg.GetActiveUserPercent())

	// 4. Deploy the test contracts which will be used in various TCs. If needed, charge tokens to test accounts.
	accGrp.DeployTestContracts(cfg.GetTcStrList(), localReservoirAccount, cfg.GetGCli(), cfg.GetChargeValue(), cfg.GetChargeParallelNum())
	if !account.IsGSRExistInRegistry(cfg.GetGCli()) && (cfg.InTheTcList("gaslessTransactionTC") || cfg.InTheTcList("gaslessRevertTransactionTC") || cfg.InTheTcList("gaslessOnlyApproveTC")) {
		log.Printf("GSR does not exist in registry, setting up liquidity and registering GSR...")

		// Charge KAIA and gasless tokens to GSRSetupManager
		localReservoirAccount.TransferSignedTxWithGuaranteeRetry(cfg.GetGCli(), account.GSRSetupManager, new(big.Int).Add(cfg.GetChargeValue(), account.GetInitialLiquidity()))
		account.GaslessTokenDeployer.SmartContractExecutionWithGuaranteeRetry(
			cfg.GetGCli(),
			accGrp.GetTestContractByName(account.ContractGaslessToken),
			account.TestContractInfos[account.ContractErc20].GenData(account.GSRSetupManager.GetAddress(), account.GetInitialLiquidity()),
		)

		// Setup liquidity
		account.SetupLiquidity(cfg.GetGCli(), accGrp)

		// Register GSR
		account.RegisterGSR(cfg.GetGCli(), accGrp, globalReservoirAccount)
	}
	accGrp.DeployTestContracts(cfg.GetTcStrList(), localReservoirAccount, cfg.GetGCli(), cfg.GetChargeValue(), cfg.GetChargeParallelNum())

	// Set SmartContractAddress value in each packages if needed
	setSmartContractAddressPerPackage(accGrp)
	return localReservoirAccount
}

func initializeTasks(cfg *config.Config, accGrp *account.AccGroup, tasks []*testcase.ExtendedTask) {
	println("Initializing tasks")

	// Tc package initializes the task
	for _, extendedTask := range tasks {
		accs := accGrp.GetAccListByName(account.AccListForSignedTx)
		if extendedTask.Name == "transferUnsignedTx" {
			accs = accGrp.GetAccListByName(account.AccListForUnsignedTx)
		} else if extendedTask.Name == "gaslessRevertTransactionTC" {
			accs = accGrp.GetAccListByName(account.AccListForGaslessRevertTx)
		} else if extendedTask.Name == "gaslessOnlyApproveTC" {
			accs = accGrp.GetAccListByName(account.AccListForGaslessApproveTx)
		}
		extendedTask.Init(accs, cfg.GetGEndpoint(), cfg.GetGasPrice())
		println("=> " + extendedTask.Name + " extendedTask is initialized.")
	}
}

func toBoomerTasks(tasks []*testcase.ExtendedTask) []*boomer.Task {
	var boomerTasks []*boomer.Task
	for _, task := range tasks {
		boomerTasks = append(boomerTasks, &boomer.Task{Weight: task.Weight, Fn: task.Fn, Name: task.Name})
	}
	return boomerTasks
}
