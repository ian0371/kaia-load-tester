package account

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const Letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var (
	gasPrice  *big.Int
	chainID   *big.Int
	baseFee   *big.Int
	batchSize int
)

type Account struct {
	id         int
	privateKey []*ecdsa.PrivateKey
	key        []string
	address    common.Address
	nonce      uint64
	balance    *big.Int
	mutex      sync.Mutex
}

func init() {
	gasPrice = big.NewInt(0)
	chainID = big.NewInt(2018)
	baseFee = big.NewInt(0)
}

func SetGasPrice(gp *big.Int) {
	gasPrice = gp
}

func SetBaseFee(bf *big.Int) {
	baseFee = bf
}

func SetChainID(id *big.Int) {
	chainID = id
}

func SetBatchSize(bs int) {
	batchSize = bs
}

func (acc *Account) Lock() {
	acc.mutex.Lock()
}

func (acc *Account) UnLock() {
	acc.mutex.Unlock()
}

func GetAccountFromKey(id int, key string) *Account {
	acc, err := crypto.HexToECDSA(key)
	if err != nil {
		log.Fatalf("Key(%v): Failed to HexToECDSA %v", key, err)
	}

	tAcc := Account{
		0,
		[]*ecdsa.PrivateKey{acc},
		[]string{key},
		crypto.PubkeyToAddress(acc.PublicKey),
		0,
		big.NewInt(0),
		sync.Mutex{},
		// make(TransactionMap),
	}

	return &tAcc
}

func (account *Account) ImportUnLockAccount(endpoint string) {
}

func NewAccount(id int) *Account {
	acc, err := crypto.GenerateKey()
	if err != nil {
		log.Fatalf("crypto.GenerateKey() : Failed to generateKey %v", err)
	}

	testKey := hex.EncodeToString(crypto.FromECDSA(acc))

	tAcc := Account{
		0,
		[]*ecdsa.PrivateKey{acc},
		[]string{testKey},
		crypto.PubkeyToAddress(acc.PublicKey),
		0,
		big.NewInt(0),
		sync.Mutex{},
		// make(TransactionMap),
	}

	return &tAcc
}

func NewKaiaAccount(id int) *Account {
	acc, err := crypto.GenerateKey()
	if err != nil {
		log.Fatalf("crypto.GenerateKey() : Failed to generateKey %v", err)
	}

	testKey := hex.EncodeToString(crypto.FromECDSA(acc))

	randomAddr := common.BytesToAddress(crypto.Keccak256([]byte(testKey))[12:])

	tAcc := Account{
		0,
		[]*ecdsa.PrivateKey{acc},
		[]string{testKey},
		randomAddr,
		0,
		big.NewInt(0),
		sync.Mutex{},
		// make(TransactionMap),
	}

	return &tAcc
}

func NewKaiaAccountWithAddr(id int, addr common.Address) *Account {
	acc, err := crypto.GenerateKey()
	if err != nil {
		log.Fatalf("crypto.GenerateKey() : Failed to generateKey %v", err)
	}

	testKey := hex.EncodeToString(crypto.FromECDSA(acc))

	tAcc := Account{
		0,
		[]*ecdsa.PrivateKey{acc},
		[]string{testKey},
		addr,
		0,
		big.NewInt(0),
		sync.Mutex{},
		// make(TransactionMap),
	}

	return &tAcc
}

func NewKaiaMultisigAccount(id int) *Account {
	k1, err := crypto.GenerateKey()
	if err != nil {
		log.Fatalf("crypto.GenerateKey() : Failed to generateKey %v", err)
	}
	k2, err := crypto.GenerateKey()
	if err != nil {
		log.Fatalf("crypto.GenerateKey() : Failed to generateKey %v", err)
	}
	k3, err := crypto.GenerateKey()
	if err != nil {
		log.Fatalf("crypto.GenerateKey() : Failed to generateKey %v", err)
	}

	testKey := hex.EncodeToString(crypto.FromECDSA(k1))

	randomAddr := common.BytesToAddress(crypto.Keccak256([]byte(testKey))[12:])

	tAcc := Account{
		0,
		[]*ecdsa.PrivateKey{k1, k2, k3},
		[]string{testKey},
		randomAddr,
		0,
		big.NewInt(0),
		sync.Mutex{},
		// make(TransactionMap),
	}

	return &tAcc
}

func (acc *Account) GetKey() *ecdsa.PrivateKey {
	return acc.privateKey[0]
}

func (acc *Account) GetAddress() common.Address {
	return acc.address
}

func (acc *Account) GetPrivateKey() string {
	return acc.key[0]
}

func (acc *Account) GetNonce(c *ethclient.Client) uint64 {
	if acc.nonce != 0 {
		return acc.nonce
	}
	ctx := context.Background()
	nonce, err := c.NonceAt(ctx, acc.GetAddress(), nil)
	if err != nil {
		log.Printf("GetNonce(): Failed to NonceAt() %v\n", err)
		return acc.nonce
	}
	acc.nonce = nonce

	// fmt.Printf("account= %v  nonce = %v\n", acc.GetAddress().String(), nonce)
	return acc.nonce
}

func (acc *Account) GetNonceFromBlock(c *ethclient.Client) uint64 {
	ctx := context.Background()
	nonce, err := c.NonceAt(ctx, acc.GetAddress(), nil)
	if err != nil {
		log.Printf("GetNonce(): Failed to NonceAt() %v\n", err)
		return acc.nonce
	}

	acc.nonce = nonce

	fmt.Printf("%v: account= %v  nonce = %v\n", os.Getpid(), acc.GetAddress().String(), nonce)
	return acc.nonce
}

func (acc *Account) UpdateNonce() {
	acc.nonce++
}

func (a *Account) GetReceipt(c *ethclient.Client, txHash common.Hash) (*types.Receipt, error) {
	ctx := context.Background()
	return c.TransactionReceipt(ctx, txHash)
}

func (a *Account) GetBalance(c *ethclient.Client) (*big.Int, error) {
	ctx := context.Background()
	balance, err := c.BalanceAt(ctx, a.GetAddress(), nil)
	if err != nil {
		return nil, err
	}
	return balance, err
}

var r = rand.New(rand.NewSource(time.Now().UnixNano()))

func (self *Account) TransferSignedTxReturnTx(withLock bool, c *ethclient.Client, to *Account, value *big.Int) (*types.Transaction, *big.Int, error) {
	if withLock {
		self.mutex.Lock()
		defer self.mutex.Unlock()
	}

	nonce := self.GetNonce(c)

	// fmt.Printf("account=%v, nonce = %v\n", self.GetAddress().String(), nonce)

	tx := types.NewTransaction(
		nonce,
		to.GetAddress(),
		value,
		21000,
		gasPrice,
		nil)
	gasPrice := tx.GasPrice()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	signer := types.LatestSignerForChainID(chainID)
	tx, err := types.SignTx(tx, signer, self.privateKey[0])
	if err != nil {
		return nil, gasPrice, err
	}
	err = c.SendTransaction(ctx, tx)
	if err != nil {
		if err.Error() == core.ErrNonceTooLow.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return tx, gasPrice, err
	}

	self.nonce++

	// fmt.Printf("%v transferSignedTx %v klay to %v klay.\n", self.GetAddress().Hex(), to.GetAddress().Hex(), value)

	return tx, gasPrice, nil
}

func (self *Account) TransferSignedTxWithGuaranteeRetry(c *ethclient.Client, to *Account, value *big.Int) *types.Transaction {
	var (
		err    error
		lastTx *types.Transaction
	)

	for {
		lastTx, _, err = self.TransferSignedTxReturnTx(true, c, to, value)
		// TODO-kaia-load-tester: return error if the error isn't able to handle
		if err == nil {
			break // Succeed, let's break the loop
		}
		log.Printf("Failed to execute: err=%s", err.Error())
		time.Sleep(1 * time.Second) // Mostly, the err is `txpool is full`, retry after a while.
		// numChargedAcc, lastFailedNum = estimateRemainingTime(accGrp, numChargedAcc, lastFailedNum)
	}

	ctx, cancelFn := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFn()

	receipt, err := bind.WaitMined(ctx, c, lastTx)
	cancelFn()
	if err != nil || (receipt != nil && receipt.Status == 0) {
		// shouldn't happen. must check if contract is correct.
		log.Fatalf("tx mined but failed, err=%s, txHash=%s", err, lastTx.Hash().String())
	}
	return lastTx
}

func (self *Account) SendSessionTx(c *ethclient.Client, sessionCtx *types.SessionContext) (common.Hash, error) {
	signer := types.LatestSignerForChainID(chainID)

	nonce := uint64(time.Now().UnixMilli())
	sessionCtx.Session.Nonce = nonce

	input, err := types.WrapTxAsInput(sessionCtx)
	if err != nil {
		return common.Hash{}, err
	}

	tx := types.NewTransaction(
		nonce,
		sessionCtx.Session.PublicKey,
		common.Big0,
		0,
		common.Big0,
		input,
	)

	tx, err = types.SignTx(tx, signer, self.privateKey[0])
	if err != nil {
		return common.Hash{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	err = c.SendTransaction(ctx, tx)
	if err != nil {
		return common.Hash{}, err
	}

	return tx.Hash(), nil
}

/*
func (self *Account) TransferNewLegacyTxWithEthBatch(c *ethclient.Client, endpoint string, to *Account, value *big.Int, input string) ([]common.Hash, *big.Int, error) {
	self.mutex.lock()
	defer self.mutex.unlock()

	var toAddress common.Address
	if to == nil {
		toAddress = common.Address{}
	} else {
		toAddress = to.GetAddress()
	}

	txs := make([]*types.Transaction, batchSize)
	for i := range batchSize {
		nonce := self.GetNonce(c)
		txs[i] = types.NewTransaction(
			nonce,
			toAddress,
			value,
			100000,
			gasPrice,
			common.FromHex(input),
		)
		self.nonce++
	}

	signer := types.LatestSignerForChainID(chainID)

	var wg sync.WaitGroup
	errChan := make(chan error, len(txs))

	for i := range txs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if err := txs[idx].SignWithKeys(signer, self.privateKey); err != nil {
				errChan <- fmt.Errorf("failed to sign tx %d: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	if err := <-errChan; err != nil {
		log.Fatalf("Failed to sign transactions: %v", err)
	}

	ctx := context.Background()
	err := c.SendTransactionBatch(ctx, txs)
	if err != nil {
		if err.Error() == core.ErrNonceTooLow.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), self.nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), self.nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), self.nonce, err)
		}
		return nil, gasPrice, err
	}

	hashes := make([]common.Hash, len(txs))
	for i := range txs {
		hashes[i] = txs[i].Hash()
	}

	return hashes, gasPrice, nil
}

func (self *Account) CheckReceiptsBatch(c *ethclient.Client, txHashes []common.Hash) ([]*types.Receipt, error) {
	ctx := context.Background()
	defer ctx.Done()

	receipts, err := c.TransactionReceiptBatch(ctx, txHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction receipts: %v", err)
	}

	for i, receipt := range receipts {
		if receipt == nil {
			return nil, fmt.Errorf("receipt not found for transaction %s", txHashes[i].Hex())
		}
		if receipt.Status != types.ReceiptStatusSuccessful {
			return nil, fmt.Errorf("transaction %s failed with status %d", txHashes[i].Hex(), receipt.Status)
		}
	}

	return receipts, nil
}
*/

func (a *Account) CheckBalance(expectedBalance *big.Int, cli *ethclient.Client) error {
	balance, _ := a.GetBalance(cli)
	if balance.Cmp(expectedBalance) != 0 {
		fmt.Println(a.address.String() + " expected : " + expectedBalance.Text(10) + " actual : " + balance.Text(10))
		return errors.New("expected : " + expectedBalance.Text(10) + " actual : " + balance.Text(10))
	}

	return nil
}

func ConcurrentTransactionSend(accs []*Account, transactionSend func(*Account)) {
	ch := make(chan int, runtime.NumCPU()*10)
	wg := sync.WaitGroup{}
	for _, acc := range accs {
		ch <- 1
		wg.Add(1)
		go func() {
			transactionSend(acc)
			<-ch
			wg.Done()
		}()
	}
	wg.Wait()
}
