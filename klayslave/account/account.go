package account

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"math/big"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	kaia "github.com/kaiachain/kaia"
	"github.com/kaiachain/kaia/accounts/abi"
	"github.com/kaiachain/kaia/accounts/abi/bind"
	"github.com/kaiachain/kaia/api"
	"github.com/kaiachain/kaia/blockchain"
	"github.com/kaiachain/kaia/blockchain/types"
	"github.com/kaiachain/kaia/blockchain/types/accountkey"
	"github.com/kaiachain/kaia/client"
	"github.com/kaiachain/kaia/common"
	"github.com/kaiachain/kaia/common/hexutil"
	"github.com/kaiachain/kaia/crypto"
	"github.com/kaiachain/kaia/kaiax/auction"
	auctionImpl "github.com/kaiachain/kaia/kaiax/auction/impl"
	"github.com/kaiachain/kaia/params"
	"github.com/kaiachain/kaia/rlp"
)

const Letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var (
	gasPrice *big.Int
	chainID  *big.Int
	baseFee  *big.Int
)

type Account struct {
	id                 int
	privateKey         []*ecdsa.PrivateKey
	key                []string
	address            common.Address
	nonce              uint64
	balance            *big.Int
	mutex              sync.Mutex
	lastBlocknumSentTx uint64
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
		0,
	}

	return &tAcc
}

func (account *Account) ImportUnLockAccount(endpoint string) {
	key := account.key[0]
	acc, err := crypto.HexToECDSA(key)
	if err != nil {
		log.Fatalf("Key(%v): Failed to HexToECDSA %v", err)
	}

	testAddr := crypto.PubkeyToAddress(acc.PublicKey)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := client.Dial(endpoint)
	if err != nil {
		log.Fatalf("ImportUnLockAccount(): Create Client %v", err)
	}

	addr, err := c.ImportRawKey(ctx, key, "")
	if err != nil {
		log.Fatalf("Account(%v) : Failed to import => %v\n", account.address, err)
	} else {
		if testAddr != addr {
			log.Fatalf("origial:%v, imported: %v\n", testAddr.String(), addr.String())
		}
	}

	res, err := c.UnlockAccount(ctx, account.address, "", 0)
	if err != nil {
		log.Fatalf("Account(%v) : Failed to Unlock: %v\n", account.address.String(), err)
	} else {
		log.Printf("Wallet UnLock Result: %v", res)
	}
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
		0,
	}

	return &tAcc
}

func NewAccountOnNode(id int, endpoint string) *Account {

	tAcc := NewAccount(id)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := client.Dial(endpoint)
	if err != nil {
		log.Fatalf("NewAccountOnNode() : Failed to create client %v", err)
	}

	addr, err := c.ImportRawKey(ctx, tAcc.key[0], "")
	if err != nil {
		//log.Printf("Account(%v) : Failed to import\n", tAcc.address, err)
	} else {
		if tAcc.address != addr {
			log.Fatalf("origial:%v, imported: %v\n", tAcc.address, addr.String())
		}
		//log.Printf("origial:%v, imported:%v\n", tAcc.address, addr.String())
	}

	_, err = c.UnlockAccount(ctx, tAcc.GetAddress(), "", 0)
	if err != nil {
		log.Printf("Account(%v) : Failed to Unlock: %v\n", tAcc.GetAddress().String(), err)
	}

	//log.Printf("Wallet UnLock Result: %v", flag)

	return tAcc
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
		0,
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
		0,
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
		0,
	}

	return &tAcc
}

func UnlockAccount(c *client.Client, addr common.Address, pwd string) {
	ctx := context.Background()
	defer ctx.Done()

	_, e := c.UnlockAccount(ctx, addr, pwd, 0)
	if e == nil {
	} else {
		fmt.Println(e)
	}
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

func (acc *Account) GetNonce(c *client.Client) uint64 {
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

	//fmt.Printf("account= %v  nonce = %v\n", acc.GetAddress().String(), nonce)
	return acc.nonce
}

func (acc *Account) GetNonceFromBlock(c *client.Client) uint64 {
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

func (a *Account) GetReceipt(c *client.Client, txHash common.Hash) (*types.Receipt, error) {
	ctx := context.Background()
	return c.TransactionReceipt(ctx, txHash)
}

func (a *Account) GetBalance(c *client.Client) (*big.Int, error) {
	ctx := context.Background()
	balance, err := c.BalanceAt(ctx, a.GetAddress(), nil)
	if err != nil {
		return nil, err
	}
	return balance, err
}

func (self *Account) TransferSignedTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	tx, gasPrice, err := self.TransferSignedTxReturnTx(true, c, to, value)
	return tx.Hash(), gasPrice, err
}

func (self *Account) TransferSignedTxWithGuaranteeRetry(c *client.Client, to *Account, value *big.Int) *types.Transaction {
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
		//numChargedAcc, lastFailedNum = estimateRemainingTime(accGrp, numChargedAcc, lastFailedNum)
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

func (self *Account) TransferSignedTxWithoutLock(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	tx, gasPrice, err := self.TransferSignedTxReturnTx(false, c, to, value)
	return tx.Hash(), gasPrice, err
}

func (self *Account) TransferSignedTxReturnTx(withLock bool, c *client.Client, to *Account, value *big.Int) (*types.Transaction, *big.Int, error) {
	if withLock {
		self.mutex.Lock()
		defer self.mutex.Unlock()
	}

	nonce := self.GetNonce(c)

	//fmt.Printf("account=%v, nonce = %v\n", self.GetAddress().String(), nonce)

	tx := types.NewTransaction(
		nonce,
		to.GetAddress(),
		value,
		21000,
		gasPrice,
		nil)
	gasPrice := tx.GasPrice()
	signTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), self.privateKey[0])
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	_, err = c.SendRawTransaction(ctx, signTx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return signTx, gasPrice, err
	}

	self.nonce++

	//fmt.Printf("%v transferSignedTx %v klay to %v klay.\n", self.GetAddress().Hex(), to.GetAddress().Hex(), value)

	return signTx, gasPrice, nil
}

func (self *Account) TransferNewValueTransferWithCancelTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	var txList []*types.Transaction
	nonce := self.GetNonce(c)

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeValueTransfer, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    nonce,
		types.TxValueKeyTo:       to.GetAddress(),
		types.TxValueKeyAmount:   value,
		types.TxValueKeyGasLimit: uint64(100000),
		types.TxValueKeyGasPrice: gasPrice,
		types.TxValueKeyFrom:     self.address,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	txList = append(txList, tx)

	cancelTx, err := types.NewTransactionWithMap(types.TxTypeCancel, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    nonce,
		types.TxValueKeyFrom:     self.address,
		types.TxValueKeyGasLimit: uint64(100000000),
		types.TxValueKeyGasPrice: gasPrice,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = cancelTx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	txList = append(txList, cancelTx)

	var hash common.Hash
	for _, tx := range txList {
		hash, err := c.SendRawTransaction(ctx, tx)
		if err != nil {
			if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
				fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
				fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
				self.nonce++
			} else {
				fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			}
			return hash, gasPrice, err
		}
	}

	self.nonce++
	return hash, gasPrice, nil
}

func (self *Account) TransferNewValueTransferTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeValueTransfer, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    nonce,
		types.TxValueKeyTo:       to.GetAddress(),
		types.TxValueKeyAmount:   value,
		types.TxValueKeyGasLimit: uint64(100000),
		types.TxValueKeyGasPrice: gasPrice,
		types.TxValueKeyFrom:     self.address,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewFeeDelegatedValueTransferTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedValueTransfer, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    nonce,
		types.TxValueKeyTo:       to.GetAddress(),
		types.TxValueKeyAmount:   value,
		types.TxValueKeyGasLimit: uint64(100000),
		types.TxValueKeyGasPrice: gasPrice,
		types.TxValueKeyFrom:     self.address,
		types.TxValueKeyFeePayer: to.address,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	err = tx.SignFeePayerWithKeys(signer, to.privateKey)
	if err != nil {
		log.Fatalf("Failed to fee payer sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewFeeDelegatedValueTransferWithRatioTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedValueTransferWithRatio, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:              nonce,
		types.TxValueKeyTo:                 to.GetAddress(),
		types.TxValueKeyAmount:             value,
		types.TxValueKeyGasLimit:           uint64(100000),
		types.TxValueKeyGasPrice:           gasPrice,
		types.TxValueKeyFrom:               self.address,
		types.TxValueKeyFeePayer:           to.address,
		types.TxValueKeyFeeRatioOfFeePayer: types.FeeRatio(30),
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	err = tx.SignFeePayerWithKeys(signer, to.privateKey)
	if err != nil {
		log.Fatalf("Failed to fee payer sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewValueTransferMemoTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)
	data := []byte("hello")

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeValueTransferMemo, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    nonce,
		types.TxValueKeyTo:       to.GetAddress(),
		types.TxValueKeyAmount:   value,
		types.TxValueKeyGasLimit: uint64(100000),
		types.TxValueKeyGasPrice: gasPrice,
		types.TxValueKeyData:     data,
		types.TxValueKeyFrom:     self.address,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func randInt(min int, max int) int {
	return min + rand.Intn(max-min)
}

// increase memo size from 5 bytes to between 50 bytes and 2,000 bytes

func (self *Account) TransferNewValueTransferBigRandomStringMemoTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)
	minBytes := 50
	maxBytes := 2000

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)
	data := randomString(randInt(minBytes, maxBytes))
	// data := []byte("hello")

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeValueTransferMemo, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    nonce,
		types.TxValueKeyTo:       to.GetAddress(),
		types.TxValueKeyAmount:   value,
		types.TxValueKeyGasLimit: uint64(100000),
		types.TxValueKeyGasPrice: gasPrice,
		types.TxValueKeyData:     data,
		types.TxValueKeyFrom:     self.address,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

// create 200 strings of memo
func (self *Account) TransferNewValueTransferSmallMemoTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)
	length := 200

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)
	data := randomString(length)
	// data := []byte("hello")

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeValueTransferMemo, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    nonce,
		types.TxValueKeyTo:       to.GetAddress(),
		types.TxValueKeyAmount:   value,
		types.TxValueKeyGasLimit: uint64(100000),
		types.TxValueKeyGasPrice: gasPrice,
		types.TxValueKeyData:     data,
		types.TxValueKeyFrom:     self.address,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

// create 2000 strings of memo
func (self *Account) TransferNewValueTransferLargeMemoTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)
	length := 2000

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)
	data := randomString(length)
	// data := []byte("hello")

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeValueTransferMemo, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    nonce,
		types.TxValueKeyTo:       to.GetAddress(),
		types.TxValueKeyAmount:   value,
		types.TxValueKeyGasLimit: uint64(100000),
		types.TxValueKeyGasPrice: gasPrice,
		types.TxValueKeyData:     data,
		types.TxValueKeyFrom:     self.address,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewFeeDelegatedValueTransferMemoTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)
	data := []byte("hello")

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedValueTransferMemo, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    nonce,
		types.TxValueKeyTo:       to.GetAddress(),
		types.TxValueKeyAmount:   value,
		types.TxValueKeyGasLimit: uint64(100000),
		types.TxValueKeyGasPrice: gasPrice,
		types.TxValueKeyData:     data,
		types.TxValueKeyFrom:     self.address,
		types.TxValueKeyFeePayer: to.address,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	err = tx.SignFeePayerWithKeys(signer, to.privateKey)
	if err != nil {
		log.Fatalf("Failed to fee payer sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewFeeDelegatedValueTransferMemoWithRatioTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)
	data := []byte("hello")

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedValueTransferMemoWithRatio, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:              nonce,
		types.TxValueKeyTo:                 to.GetAddress(),
		types.TxValueKeyAmount:             value,
		types.TxValueKeyGasLimit:           uint64(100000),
		types.TxValueKeyGasPrice:           gasPrice,
		types.TxValueKeyData:               data,
		types.TxValueKeyFrom:               self.address,
		types.TxValueKeyFeePayer:           to.address,
		types.TxValueKeyFeeRatioOfFeePayer: types.FeeRatio(30),
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	err = tx.SignFeePayerWithKeys(signer, to.privateKey)
	if err != nil {
		log.Fatalf("Failed to fee payer sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewAccountCreationTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeAccountCreation, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:         nonce,
		types.TxValueKeyFrom:          self.address,
		types.TxValueKeyTo:            to.GetAddress(),
		types.TxValueKeyAmount:        value,
		types.TxValueKeyGasLimit:      uint64(1000000),
		types.TxValueKeyGasPrice:      gasPrice,
		types.TxValueKeyHumanReadable: false,
		types.TxValueKeyAccountKey:    accountkey.NewAccountKeyPublicWithValue(&to.privateKey[0].PublicKey),
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewAccountUpdateTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeAccountUpdate, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:      nonce,
		types.TxValueKeyFrom:       self.address,
		types.TxValueKeyGasLimit:   uint64(100000),
		types.TxValueKeyGasPrice:   gasPrice,
		types.TxValueKeyAccountKey: accountkey.NewAccountKeyPublicWithValue(&self.privateKey[0].PublicKey),
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewFeeDelegatedAccountUpdateTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedAccountUpdate, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:      nonce,
		types.TxValueKeyFrom:       self.address,
		types.TxValueKeyGasLimit:   uint64(100000),
		types.TxValueKeyGasPrice:   gasPrice,
		types.TxValueKeyAccountKey: accountkey.NewAccountKeyPublicWithValue(&self.privateKey[0].PublicKey),
		types.TxValueKeyFeePayer:   to.address,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	err = tx.SignFeePayerWithKeys(signer, to.privateKey)
	if err != nil {
		log.Fatalf("Failed to fee payer sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewFeeDelegatedAccountUpdateWithRatioTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedAccountUpdateWithRatio, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:              nonce,
		types.TxValueKeyFrom:               self.address,
		types.TxValueKeyGasLimit:           uint64(100000),
		types.TxValueKeyGasPrice:           gasPrice,
		types.TxValueKeyAccountKey:         accountkey.NewAccountKeyPublicWithValue(&self.privateKey[0].PublicKey),
		types.TxValueKeyFeePayer:           to.address,
		types.TxValueKeyFeeRatioOfFeePayer: types.FeeRatio(30),
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	err = tx.SignFeePayerWithKeys(signer, to.privateKey)
	if err != nil {
		log.Fatalf("Failed to fee payer sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewSmartContractDeployTx(c *client.Client, to *Account, value *big.Int, data []byte) (common.Address, *types.Transaction, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)
	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeSmartContractDeploy, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:         nonce,
		types.TxValueKeyFrom:          self.address,
		types.TxValueKeyTo:            (*common.Address)(nil),
		types.TxValueKeyAmount:        value,
		types.TxValueKeyGasLimit:      uint64(10000000),
		types.TxValueKeyGasPrice:      gasPrice,
		types.TxValueKeyHumanReadable: false,
		types.TxValueKeyCodeFormat:    params.CodeFormatEVM,
		types.TxValueKeyData:          data,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	_, err = c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return common.Address{}, tx, gasPrice, err
	}

	contractAddr := crypto.CreateAddress(self.address, self.nonce)

	self.nonce++

	return contractAddr, tx, gasPrice, nil
}

func (self *Account) TransferNewFeeDelegatedSmartContractDeployTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	code := "0x608060405234801561001057600080fd5b506101de806100206000396000f3006080604052600436106100615763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416631a39d8ef81146100805780636353586b146100a757806370a08231146100ca578063fd6b7ef8146100f8575b3360009081526001602052604081208054349081019091558154019055005b34801561008c57600080fd5b5061009561010d565b60408051918252519081900360200190f35b6100c873ffffffffffffffffffffffffffffffffffffffff60043516610113565b005b3480156100d657600080fd5b5061009573ffffffffffffffffffffffffffffffffffffffff60043516610147565b34801561010457600080fd5b506100c8610159565b60005481565b73ffffffffffffffffffffffffffffffffffffffff1660009081526001602052604081208054349081019091558154019055565b60016020526000908152604090205481565b336000908152600160205260408120805490829055908111156101af57604051339082156108fc029083906000818181858888f193505050501561019c576101af565b3360009081526001602052604090208190555b505600a165627a7a72305820627ca46bb09478a015762806cc00c431230501118c7c26c30ac58c4e09e51c4f0029"

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedSmartContractDeploy, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:         nonce,
		types.TxValueKeyFrom:          self.address,
		types.TxValueKeyTo:            &to.address,
		types.TxValueKeyAmount:        common.Big0,
		types.TxValueKeyGasLimit:      uint64(10000000),
		types.TxValueKeyGasPrice:      gasPrice,
		types.TxValueKeyHumanReadable: false,
		types.TxValueKeyData:          common.FromHex(code),
		types.TxValueKeyCodeFormat:    params.CodeFormatEVM,
		types.TxValueKeyFeePayer:      self.address,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	err = tx.SignFeePayerWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to fee payer sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewFeeDelegatedSmartContractDeployWithRatioTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	code := "0x608060405234801561001057600080fd5b506101de806100206000396000f3006080604052600436106100615763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416631a39d8ef81146100805780636353586b146100a757806370a08231146100ca578063fd6b7ef8146100f8575b3360009081526001602052604081208054349081019091558154019055005b34801561008c57600080fd5b5061009561010d565b60408051918252519081900360200190f35b6100c873ffffffffffffffffffffffffffffffffffffffff60043516610113565b005b3480156100d657600080fd5b5061009573ffffffffffffffffffffffffffffffffffffffff60043516610147565b34801561010457600080fd5b506100c8610159565b60005481565b73ffffffffffffffffffffffffffffffffffffffff1660009081526001602052604081208054349081019091558154019055565b60016020526000908152604090205481565b336000908152600160205260408120805490829055908111156101af57604051339082156108fc029083906000818181858888f193505050501561019c576101af565b3360009081526001602052604090208190555b505600a165627a7a72305820627ca46bb09478a015762806cc00c431230501118c7c26c30ac58c4e09e51c4f0029"

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedSmartContractDeployWithRatio, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:              nonce,
		types.TxValueKeyFrom:               self.address,
		types.TxValueKeyTo:                 &to.address,
		types.TxValueKeyAmount:             common.Big0,
		types.TxValueKeyGasLimit:           uint64(10000000),
		types.TxValueKeyGasPrice:           gasPrice,
		types.TxValueKeyHumanReadable:      false,
		types.TxValueKeyData:               common.FromHex(code),
		types.TxValueKeyFeePayer:           self.address,
		types.TxValueKeyCodeFormat:         params.CodeFormatEVM,
		types.TxValueKeyFeeRatioOfFeePayer: types.FeeRatio(30),
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	err = tx.SignFeePayerWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to fee payer sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

var r = rand.New(rand.NewSource(time.Now().UnixNano()))

func randomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = Letters[r.Intn(len(Letters))]
	}
	return string(b)
}

func (self *Account) ExecuteStorageTrieStore(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	r = rand.New(rand.NewSource(time.Now().UnixNano()))
	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	abiStr := `[{"constant":true,"inputs":[],"name":"rootCaCertificate","outputs":[{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[{"name":"_serialNumber","type":"string"}],"name":"getIdentity","outputs":[{"name":"","type":"string"},{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"name":"_caKey","type":"string"}],"name":"deleteCaCertificate","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":false,"inputs":[{"name":"_caKey","type":"string"},{"name":"_caCert","type":"string"}],"name":"insertCaCertificate","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"owner","outputs":[{"name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"name":"_serialNumber","type":"string"},{"name":"_publicKey","type":"string"},{"name":"_hash","type":"string"}],"name":"insertIdentity","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":false,"inputs":[{"name":"_serialNumber","type":"string"}],"name":"deleteIdentity","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[{"name":"_caKey","type":"string"}],"name":"getCaCertificate","outputs":[{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},{"inputs":[],"payable":false,"stateMutability":"nonpayable","type":"constructor"}]`

	abii, err := abi.JSON(strings.NewReader(string(abiStr)))
	if err != nil {
		log.Fatalf("failed to abi.JSON: %v", err)
	}
	data, err := abii.Pack("insertIdentity", randomString(39), randomString(814), randomString(40))
	if err != nil {
		log.Fatalf("failed to abi.Pack: %v", err)
	}

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeSmartContractExecution, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    nonce,
		types.TxValueKeyGasPrice: gasPrice,
		types.TxValueKeyGasLimit: uint64(5000000),
		types.TxValueKeyFrom:     self.address,
		types.TxValueKeyAmount:   common.Big0,
		types.TxValueKeyTo:       to.address,
		types.TxValueKeyData:     data,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	// log.Printf("data %s", common.Bytes2Hex(data))
	// log.Printf("to.address %s", to.address.String())
	// log.Printf("tx %s\n", tx.String())

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewSmartContractExecutionTx(c *client.Client, to *Account, value *big.Int, data []byte) (*types.Transaction, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	if value == nil {
		value = big.NewInt(0)
	}

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeSmartContractExecution, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    nonce,
		types.TxValueKeyGasPrice: gasPrice,
		types.TxValueKeyGasLimit: uint64(5000000),
		types.TxValueKeyFrom:     self.address,
		types.TxValueKeyAmount:   value,
		types.TxValueKeyTo:       to.address,
		types.TxValueKeyData:     data,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	_, err = c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return tx, gasPrice, err
	}

	self.nonce++

	return tx, gasPrice, nil
}

func (self *Account) TransferNewFeeDelegatedSmartContractExecutionTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)
	abiStr := `[{"constant":true,"inputs":[],"name":"totalAmount","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"name":"receiver","type":"address"}],"name":"reward","outputs":[],"payable":true,"stateMutability":"payable","type":"function"},{"constant":true,"inputs":[{"name":"","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[],"name":"safeWithdrawal","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"inputs":[],"payable":false,"stateMutability":"nonpayable","type":"constructor"},{"payable":true,"stateMutability":"payable","type":"fallback"}]`

	abii, err := abi.JSON(strings.NewReader(string(abiStr)))
	if err != nil {
		log.Fatalf("failed to abi.JSON: %v", err)
	}

	data, err := abii.Pack("reward", self.address)
	if err != nil {
		log.Fatalf("failed to abi.Pack: %v", err)
	}

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedSmartContractExecution, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    nonce,
		types.TxValueKeyGasPrice: gasPrice,
		types.TxValueKeyGasLimit: uint64(5000000),
		types.TxValueKeyFrom:     self.address,
		types.TxValueKeyAmount:   value,
		types.TxValueKeyTo:       to.address,
		types.TxValueKeyData:     data,
		types.TxValueKeyFeePayer: self.address,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	err = tx.SignFeePayerWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to fee payer sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewFeeDelegatedSmartContractExecutionWithRatioTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)
	abiStr := `[{"constant":true,"inputs":[],"name":"totalAmount","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"name":"receiver","type":"address"}],"name":"reward","outputs":[],"payable":true,"stateMutability":"payable","type":"function"},{"constant":true,"inputs":[{"name":"","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[],"name":"safeWithdrawal","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"inputs":[],"payable":false,"stateMutability":"nonpayable","type":"constructor"},{"payable":true,"stateMutability":"payable","type":"fallback"}]`

	abii, err := abi.JSON(strings.NewReader(string(abiStr)))
	if err != nil {
		log.Fatalf("failed to abi.JSON: %v", err)
	}

	data, err := abii.Pack("reward", self.address)
	if err != nil {
		log.Fatalf("failed to abi.Pack: %v", err)
	}

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedSmartContractExecutionWithRatio, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:              nonce,
		types.TxValueKeyGasPrice:           gasPrice,
		types.TxValueKeyGasLimit:           uint64(5000000),
		types.TxValueKeyFrom:               self.address,
		types.TxValueKeyAmount:             value,
		types.TxValueKeyTo:                 to.address,
		types.TxValueKeyData:               data,
		types.TxValueKeyFeePayer:           self.address,
		types.TxValueKeyFeeRatioOfFeePayer: types.FeeRatio(30),
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	err = tx.SignFeePayerWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to fee payer sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewCancelTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeCancel, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    nonce,
		types.TxValueKeyFrom:     self.address,
		types.TxValueKeyGasLimit: uint64(100000000),
		types.TxValueKeyGasPrice: gasPrice,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewFeeDelegatedCancelTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedCancel, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    nonce,
		types.TxValueKeyFrom:     self.address,
		types.TxValueKeyGasLimit: uint64(100000000),
		types.TxValueKeyGasPrice: gasPrice,
		types.TxValueKeyFeePayer: to.address,
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	err = tx.SignFeePayerWithKeys(signer, to.privateKey)
	if err != nil {
		log.Fatalf("Failed to fee payer sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewFeeDelegatedCancelWithRatioTx(c *client.Client, to *Account, value *big.Int) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	signer := types.NewEIP155Signer(chainID)
	tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedCancelWithRatio, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:              nonce,
		types.TxValueKeyFrom:               self.address,
		types.TxValueKeyGasLimit:           uint64(100000000),
		types.TxValueKeyGasPrice:           gasPrice,
		types.TxValueKeyFeePayer:           to.address,
		types.TxValueKeyFeeRatioOfFeePayer: types.FeeRatio(30),
	})
	if err != nil {
		log.Fatalf("Failed to encode tx: %v", err)
	}

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	err = tx.SignFeePayerWithKeys(signer, to.privateKey)
	if err != nil {
		log.Fatalf("Failed to fee payer sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewEthereumAccessListTx(c *client.Client, to *Account, value *big.Int, input []byte) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	gas := uint64(5000000)

	var toAddress *common.Address
	if to != nil {
		toAddress = &to.address
	}
	callMsg := kaia.CallMsg{
		From:     self.address,
		To:       toAddress,
		Gas:      gas,
		GasPrice: gasPrice,
		Value:    value,
		Data:     input,
	}
	accessList, _, _, err := c.CreateAccessList(ctx, callMsg)
	if err != nil {
		log.Fatalf("Failed to get accessList: %v", err)
	}

	signer := types.LatestSignerForChainID(chainID)

	tx := types.NewTx(&types.TxInternalDataEthereumAccessList{
		ChainID:      chainID,
		AccountNonce: nonce,
		Recipient:    toAddress,
		GasLimit:     gas,
		Price:        gasPrice,
		Amount:       value,
		AccessList:   *accessList,
		Payload:      input,
	})

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewEthereumDynamicFeeTx(c *client.Client, to *Account, value *big.Int, input []byte) (common.Hash, *big.Int, error) {
	ctx := context.Background() //context.WithTimeout(context.Background(), 100*time.Second)

	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	gas := uint64(5000000)

	var toAddress *common.Address
	if to != nil {
		toAddress = &to.address
	}
	callMsg := kaia.CallMsg{
		From:     self.address,
		To:       toAddress,
		Gas:      gas,
		GasPrice: gasPrice,
		Value:    value,
		Data:     input,
	}
	accessList, _, _, err := c.CreateAccessList(ctx, callMsg)
	if err != nil {
		log.Fatalf("Failed to get accessList: %v", err)
	}

	signer := types.LatestSignerForChainID(chainID)

	tx := types.NewTx(&types.TxInternalDataEthereumDynamicFee{
		ChainID:      chainID,
		AccountNonce: nonce,
		Recipient:    toAddress,
		GasLimit:     gas,
		GasFeeCap:    gasPrice,
		GasTipCap:    gasPrice,
		Amount:       value,
		AccessList:   *accessList,
		Payload:      input,
	})

	err = tx.SignWithKeys(signer, self.privateKey)
	if err != nil {
		log.Fatalf("Failed to sign tx: %v", err)
	}

	hash, err := c.SendRawTransaction(ctx, tx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return hash, gasPrice, err
	}

	self.nonce++

	return hash, gasPrice, nil
}

func (self *Account) TransferNewLegacyTxWithEth(c *client.Client, endpoint string, to *Account, value *big.Int, input string, exePath string) (common.Hash, *big.Int, error) {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	// Ethereum LegacyTx
	txType := "0"
	gas := "100000"

	var toAddress string
	if to != nil {
		toAddress = to.GetAddress().String()
	} else {
		// When to is nil, smart contract deployment with legacyTx case.
		// To send as a command argument which has to be string type,
		// explicitly send "nil" string for deploying.
		toAddress = "nil"
		gas = "200000"
	}

	// To test this, you need to update submodule and build executable file.
	// ./ethTxGenerator endPoint txType chainID gasPrice gas baseFee value fromPrivateKey nonce to [data]
	cmd := exec.Command(exePath, endpoint, txType, chainID.String(), gasPrice.String(), gas, baseFee.String(), value.String(), self.GetPrivateKey(), strconv.FormatUint(nonce, 10), toAddress, input)
	result, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to create and send tx : %v", err)
	}

	strResult := string(result[:])
	// Executable file will return transaction hash or error string.
	// So if result does not include "0x" prefix, means something went wrong.
	if !strings.Contains(strResult, "0x") {
		err = errors.New(strResult)
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return common.Hash{0}, gasPrice, err
	}

	self.nonce++

	return common.HexToHash(strResult), gasPrice, nil
}

// This function is responsible for sending both Gasless Approve Transactions and Gasless Swap Transactions.
func (self *Account) TransferNewGaslessTx(c *client.Client, endpoint string, testToken, gsr *Account) (common.Hash, common.Hash, *big.Int, error) {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	ctx := context.Background()
	suggestedGasPrice, err := c.SuggestGasPrice(ctx)
	if err != nil {
		fmt.Printf("Failed to fetch suggest gas price: %v\n", err.Error())
		return common.Hash{0}, common.Hash{0}, gasPrice, err
	}

	approveTx := types.NewTransaction(
		nonce,
		testToken.address,
		common.Big0,
		100000,
		suggestedGasPrice,
		TestContractInfos[ContractGaslessToken].GenData(gsr.GetAddress(), abi.MaxUint256)) // Approve maximum amount
	signApproveTx, err := types.SignTx(approveTx, types.NewEIP155Signer(chainID), self.privateKey[0])
	if err != nil {
		log.Fatalf("Failed to encode approve tx: %v", err)
	}

	swapTx := types.NewTransaction(
		nonce+1,
		gsr.address,
		common.Big0,
		500000,
		suggestedGasPrice,
		TestContractInfos[ContractGaslessSwapRouter].GenData(testToken.GetAddress(), suggestedGasPrice))
	signSwapTx, err := types.SignTx(swapTx, types.NewEIP155Signer(chainID), self.privateKey[0])
	if err != nil {
		log.Fatalf("Failed to encode swap tx: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	_, err = c.SendRawTransaction(ctx, signApproveTx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return approveTx.Hash(), swapTx.Hash(), suggestedGasPrice, err
	}

	_, err = c.SendRawTransaction(ctx, signSwapTx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return approveTx.Hash(), swapTx.Hash(), suggestedGasPrice, err
	}

	self.nonce += 2

	return approveTx.Hash(), swapTx.Hash(), suggestedGasPrice, nil
}

// This function is responsible for sending only Gasless Approve Transactions.
// This function won't increase nonce.
func (self *Account) TransferNewGaslessApproveTx(c *client.Client, endpoint string, testToken, gsr *Account) (common.Hash, *big.Int, error) {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	ctx := context.Background()
	suggestedGasPrice, err := c.SuggestGasPrice(ctx)
	if err != nil {
		fmt.Printf("Failed to fetch suggest gas price: %v\n", err.Error())
		return common.Hash{0}, gasPrice, err
	}

	approveTx := types.NewTransaction(
		nonce,
		testToken.address,
		common.Big0,
		100000,
		suggestedGasPrice,
		TestContractInfos[ContractGaslessToken].GenData(gsr.GetAddress(), abi.MaxUint256)) // Approve maximum amount
	signApproveTx, err := types.SignTx(approveTx, types.NewEIP155Signer(chainID), self.privateKey[0])
	if err != nil {
		log.Fatalf("Failed to encode approve tx: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	_, err = c.SendRawTransaction(ctx, signApproveTx)
	if err != nil {
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return approveTx.Hash(), suggestedGasPrice, err
	}

	return approveTx.Hash(), suggestedGasPrice, nil
}

func (self *Account) AuctionBid(c *client.Client, endpoint string, auctionEntryPoint, targetContract *Account, targetTxTypeKey string) (common.Hash, common.Hash, *big.Int, error) {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	// create tmpAccount
	tmpAccount := NewAccount(0)

	/* ---------------- Generate target tx ---------------- */
	nonce := self.GetNonce(c)
	ctx := context.Background()
	suggestedGasPrice, err := c.SuggestGasPrice(ctx)
	if err != nil {
		fmt.Printf("Failed to fetch suggest gas price: %v\n", err.Error())
		return common.Hash{0}, common.Hash{0}, gasPrice, err
	}

	targetTxType := TargetTxTypeList[targetTxTypeKey]
	targetTx := targetTxType.GenerateTx(c, self, tmpAccount, nonce, suggestedGasPrice)
	if targetTx == nil {
		return common.Hash{0}, common.Hash{0}, suggestedGasPrice, errors.New("failed to generate target tx")
	}

	/* ---------------- Send bid -------------------------- */
	err = targetTxType.PreSendBid(c, self, tmpAccount, nonce, suggestedGasPrice)
	if err != nil {
		// If PreSendBid fails, the remaining steps do not need to be performed.
		return common.Hash{0}, common.Hash{0}, suggestedGasPrice, err
	}

	gas := uint64(5000000)

	// Get entrypoint nonce
	appNonce := getEntrypointNonce(c, self.GetAddress())

	// Create contract call data (CounterForAuction.incForAuction())
	contractCallData := TestContractInfos[ContractCounterForTestAuction].GenData(common.Address{}, common.Big0) // 0 means calling incForAuction()

	// Get current block number
	blockNumber, err := c.BlockNumber(ctx)
	if err != nil {
		return common.Hash{0}, common.Hash{0}, suggestedGasPrice, err
	}
	if self.isLastBlocknumSentTx(blockNumber.Uint64()) {
		return common.Hash{0}, common.Hash{0}, suggestedGasPrice, errors.New("this account has already sent a tx for the block")
	}

	// Create the bid
	bid := &auction.Bid{
		BidData: auction.BidData{
			TargetTxHash: targetTx.Hash(),
			BlockNumber:  new(big.Int).Add(blockNumber, common.Big1).Uint64(),
			Sender:       self.address,
			To:           targetContract.address,
			Nonce:        appNonce.Uint64(),
			Bid:          big.NewInt(2),
			CallGasLimit: gas,
			Data:         contractCallData,
		},
	}

	// searcher sign bid
	searcherSignedBid, err := self.signAuctionBidAsSearcher(bid, auctionEntryPoint)
	if err != nil {
		return common.Hash{0}, common.Hash{0}, suggestedGasPrice, err
	}

	// auctioneer sign bid
	bidInput, err := Auctioneer.signAuctionBidAsAuctioneer(searcherSignedBid, toRlp(targetTx))
	if err != nil {
		return common.Hash{0}, common.Hash{0}, suggestedGasPrice, err
	}

	// send bid
	var submitErr string
	rpcOutput, err := c.SendAuctionTx(ctx, *bidInput)
	if err != nil {
		return targetTx.Hash(), bid.Hash(), suggestedGasPrice, err
	}

	/* ---------------- Handle rpc output -------------------------- */
	if rpcOutput[auctionImpl.RPC_AUCTION_ERROR_PROP] != nil {
		submitErr = rpcOutput[auctionImpl.RPC_AUCTION_ERROR_PROP].(string)
	}
	if submitErr != "" {
		if submitErr == blockchain.ErrNonceTooLow.Error() || submitErr == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, submitErr)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		}
		return targetTx.Hash(), bid.Hash(), suggestedGasPrice, errors.New(submitErr)
	}

	targetTxType.PostSendBid(c, self, tmpAccount, nonce, suggestedGasPrice, blockNumber)

	return targetTx.Hash(), bid.Hash(), suggestedGasPrice, nil
}

// AuctionRevertedBid is responsible for sending reverted bid.
// Using an invalid nonce in a bid will cause the bid tx to be reverted.
func (self *Account) AuctionRevertedBid(c *client.Client, endpoint string, auctionEntryPoint, targetContract *Account, targetTxTypeKey string) (common.Hash, common.Hash, *big.Int, error) {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	// create tmpAccount
	tmpAccount := NewAccount(0)

	/* ---------------- Generate target tx ---------------- */
	nonce := self.GetNonce(c)
	ctx := context.Background()
	suggestedGasPrice, err := c.SuggestGasPrice(ctx)
	if err != nil {
		fmt.Printf("Failed to fetch suggest gas price: %v\n", err.Error())
		return common.Hash{0}, common.Hash{0}, gasPrice, err
	}

	targetTxType := TargetTxTypeList[targetTxTypeKey]
	targetTx := targetTxType.GenerateTx(c, self, tmpAccount, nonce, suggestedGasPrice)
	if targetTx == nil {
		return common.Hash{0}, common.Hash{0}, suggestedGasPrice, errors.New("failed to generate target tx")
	}

	/* ---------------- Send bid -------------------------- */
	err = targetTxType.PreSendBid(c, self, tmpAccount, nonce, suggestedGasPrice)
	if err != nil {
		// If PreSendBid fails, the remaining steps do not need to be performed.
		return common.Hash{0}, common.Hash{0}, suggestedGasPrice, err
	}

	gas := uint64(5000000)

	// Create contract call data (CounterForAuction.incForAuction())
	contractCallData := TestContractInfos[ContractCounterForTestAuction].GenData(common.Address{}, common.Big0) // 0 means calling incForAuction()

	// Get current block number
	blockNumber, err := c.BlockNumber(ctx)
	if err != nil {
		return common.Hash{0}, common.Hash{0}, suggestedGasPrice, err
	}
	if self.isLastBlocknumSentTx(blockNumber.Uint64()) {
		return common.Hash{0}, common.Hash{0}, suggestedGasPrice, errors.New("this account has already sent a tx for the block")
	}

	// Create the bid
	bid := &auction.Bid{
		BidData: auction.BidData{
			TargetTxHash: targetTx.Hash(),
			BlockNumber:  new(big.Int).Add(blockNumber, common.Big1).Uint64(),
			Sender:       self.address,
			To:           targetContract.address,
			Nonce:        math.MaxUint64, // This causes a revert.
			Bid:          big.NewInt(2),
			CallGasLimit: gas,
			Data:         contractCallData,
		},
	}

	// searcher sign bid
	searcherSignedBid, err := self.signAuctionBidAsSearcher(bid, auctionEntryPoint)
	if err != nil {
		return common.Hash{0}, common.Hash{0}, suggestedGasPrice, err
	}

	// auctioneer sign bid
	bidInput, err := Auctioneer.signAuctionBidAsAuctioneer(searcherSignedBid, toRlp(targetTx))
	if err != nil {
		return common.Hash{0}, common.Hash{0}, suggestedGasPrice, err
	}

	// send bid
	var submitErr string
	rpcOutput, err := c.SendAuctionTx(ctx, *bidInput)
	if err != nil {
		return targetTx.Hash(), bid.Hash(), suggestedGasPrice, err
	}

	/* ---------------- Handle rpc output -------------------------- */
	if rpcOutput[auctionImpl.RPC_AUCTION_ERROR_PROP] != nil {
		submitErr = rpcOutput[auctionImpl.RPC_AUCTION_ERROR_PROP].(string)
	}
	if submitErr != "" {
		if submitErr == blockchain.ErrNonceTooLow.Error() || submitErr == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, submitErr)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		}
		return targetTx.Hash(), bid.Hash(), suggestedGasPrice, errors.New(submitErr)
	}

	targetTxType.PostSendBid(c, self, tmpAccount, nonce, suggestedGasPrice, blockNumber)

	return targetTx.Hash(), bid.Hash(), suggestedGasPrice, nil
}

func (self *Account) TransferNewEthAccessListTxWithEth(c *client.Client, endpoint string, to *Account, value *big.Int, input string, exePath string) (common.Hash, *big.Int, error) {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	// Ethereum AccessListTx
	txType := "1"
	gas := "100000"

	var toAddress string
	if to != nil {
		toAddress = to.GetAddress().String()
	} else {
		// When to is nil, smart contract deployment with legacyTx case.
		// To send as a command argument which has to be string type,
		// explicitly send "nil" string for deploying.
		toAddress = "nil"
		gas = "200000"
	}

	// To test this, you need to update submodule and build executable file.
	// ./ethTxGenerator endPoint txType chainID gasPrice gas baseFee value fromPrivateKey nonce to [data]
	cmd := exec.Command(exePath, endpoint, txType, chainID.String(), gasPrice.String(), gas, baseFee.String(), value.String(), self.GetPrivateKey(), strconv.FormatUint(nonce, 10), toAddress, input)
	result, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to create and send tx : %v", err)
	}

	strResult := string(result[:])
	// Executable file will return transaction hash or error string.
	// So if result does not include "0x" prefix, means something went wrong.
	if !strings.Contains(strResult, "0x") {
		err = errors.New(strResult)
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return common.Hash{0}, gasPrice, err
	}

	self.nonce++

	return common.HexToHash(strResult), gasPrice, nil
}

func (self *Account) TransferNewEthDynamicFeeTxWithEth(c *client.Client, endpoint string, to *Account, value *big.Int, input string, exePath string) (common.Hash, *big.Int, error) {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	nonce := self.GetNonce(c)

	// Ethereum DynamicFeeTx
	txType := "2"
	gas := "100000"

	var toAddress string
	if to != nil {
		toAddress = to.GetAddress().String()
	} else {
		// When to is nil, smart contract deployment with legacyTx case.
		// To send as a command argument which has to be string type,
		// explicitly send "nil" string for deploying.
		toAddress = "nil"
		gas = "200000"
	}

	// To test this, you need to update submodule and build executable file.
	// ./ethTxGenerator endPoint txType chainID gasPrice gas baseFee value fromPrivateKey nonce to [data]
	cmd := exec.Command(exePath, endpoint, txType, chainID.String(), gasPrice.String(), gas, baseFee.String(), value.String(), self.GetPrivateKey(), strconv.FormatUint(nonce, 10), toAddress, input)
	result, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("fromAddress: %v, strconv.FormatUint(nonce, 10): %v, to: %v input: %v gas: %v \n", self.GetAddress().String(), strconv.FormatUint(nonce, 10), toAddress, input, gas)
		log.Fatalf("Failed to create and send tx : %v", err)
	}

	strResult := string(result[:])
	// Executable file will return transaction hash or error string.
	// So if result does not include "0x" prefix, means something went wrong.
	if !strings.Contains(strResult, "0x") {
		err = errors.New(strResult)
		if err.Error() == blockchain.ErrNonceTooLow.Error() || err.Error() == blockchain.ErrReplaceUnderpriced.Error() {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
			fmt.Printf("Account(%v) nonce is added to %v\n", self.GetAddress().String(), nonce+1)
			self.nonce++
		} else {
			fmt.Printf("Account(%v) nonce(%v) : Failed to sendTransaction: %v\n", self.GetAddress().String(), nonce, err)
		}
		return common.Hash{0}, gasPrice, err
	}

	self.nonce++

	return common.HexToHash(strResult), gasPrice, nil
}

func (self *Account) TransferUnsignedTx(c *client.Client, to *Account, value *big.Int) (common.Hash, error) {
	ctx := context.Background()

	fromAddr := self.GetAddress()
	toAddr := to.GetAddress()
	gasLimit := hexutil.Uint64(21000)

	var err error
	hash, err := c.SendUnsignedTransaction(ctx, api.SendTxArgs{
		From:      fromAddr,
		Recipient: &toAddr,
		GasLimit:  &gasLimit,
		Price:     (*hexutil.Big)(gasPrice),
		Amount:    (*hexutil.Big)(value),
	})
	if err != nil {
		log.Printf("Account(%v) : Failed to sendTransaction: %v\n", self.address[:5], err)
		return common.Hash{}, err
	}
	//log.Printf("Account(%v) : Success to sendTransaction: %v\n", self.address[:5], hash.String())
	return hash, nil
}

// SmartContractDeployWithGuaranteeRetry deploys only one smart contract among the slaves.
// It the contract is already deployed by other slave, it just calculates the address of the contract.
func (self *Account) SmartContractDeployWithGuaranteeRetry(gCli *client.Client, byteCode []byte, contractName string) *Account {
	log.Println(contractName, "deployer", self.address.String())

	var (
		err    error
		addr   common.Address
		lastTx *types.Transaction
	)

	nonce := self.GetNonce(gCli)

	for {
		addr, lastTx, _, err = self.TransferNewSmartContractDeployTx(gCli, nil, common.Big0, byteCode)
		if err == nil || strings.HasPrefix(err.Error(), "known transaction") {
			break
		}
		log.Printf("Failed to deploy a %s: err %s", contractName, err.Error())
		time.Sleep(5 * time.Second) // Mostly, the err is `txpool is full`, retry after a while.
	}

	log.Printf("Start waiting the receipt of the tx(%v).\n", lastTx.Hash().String())
	ctx, cancelFn := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFn()

	receipt, err := bind.WaitMined(ctx, gCli, lastTx)
	if err != nil || (receipt != nil && receipt.Status == 0) {
		// shouldn't happen. must check if contract is correct.
		log.Fatalf("tx mined but failed, err=%s, receipt=%s", err, receipt)
		return nil
	}

	log.Printf("%s has been deployed to : %s\n", contractName, addr.String())
	return NewKaiaAccountWithAddr(1, crypto.CreateAddress(self.GetAddress(), nonce))
}

// TODO-kaia-load-tester: unify Retry functions into one function
func (a *Account) SmartContractExecutionWithGuaranteeRetry(gCli *client.Client, to *Account, value *big.Int, data []byte) {
	var (
		err    error
		lastTx *types.Transaction
	)

	for {
		lastTx, _, err = a.TransferNewSmartContractExecutionTx(gCli, to, value, data)
		if err == nil {
			break
		}
		log.Printf("Failed to execute: err=%s", err.Error())
		time.Sleep(1 * time.Second) // Mostly, the err is `txpool is full`, retry after a while.
	}
	ctx, cancelFn := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFn()

	receipt, err := bind.WaitMined(ctx, gCli, lastTx)
	cancelFn()
	if err != nil || (receipt != nil && receipt.Status == 0) {
		// shouldn't happen. must check if contract is correct.
		log.Fatalf("tx mined but failed, err=%s, txHash=%s", err, lastTx.Hash().String())
	}
}

func (a *Account) TryRunTxSendFunctionWithGuaranteeRetry(gCli *client.Client, allowedErrors []error, txSendFunc func(gCli *client.Client, sender *Account) (*types.Transaction, error)) {
	var (
		err    error
		lastTx *types.Transaction
	)

	for {
		lastTx, err = txSendFunc(gCli, a)
		if err == nil {
			break
		}

		for _, allowError := range allowedErrors {
			if err.Error() == allowError.Error() {
				log.Printf("Skipping the transaction: err=%s", err.Error())
				return
			}
		}

		log.Printf("Failed to send tx: err=%s", err.Error())
		time.Sleep(1 * time.Second)
	}
	ctx, cancelFn := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFn()

	receipt, err := bind.WaitMined(ctx, gCli, lastTx)
	cancelFn()
	if err != nil || (receipt != nil && receipt.Status == 0) {
		// shouldn't happen. must check if contract is correct.
		log.Fatalf("tx mined but failed, err=%s, txHash=%s", err, lastTx.Hash().String())
	}
}

func (a *Account) CheckBalance(expectedBalance *big.Int, cli *client.Client) error {
	balance, _ := a.GetBalance(cli)
	if balance.Cmp(expectedBalance) != 0 {
		fmt.Println(a.address.String() + " expected : " + expectedBalance.Text(10) + " actual : " + balance.Text(10))
		return errors.New("expected : " + expectedBalance.Text(10) + " actual : " + balance.Text(10))
	}

	return nil
}

func (a *Account) isLastBlocknumSentTx(blockNumber uint64) bool {
	return a.lastBlocknumSentTx == blockNumber
}

func (a *Account) updateLastBlocknumSentTx(blockNumber uint64) {
	a.lastBlocknumSentTx = blockNumber
}

func (a *Account) signAuctionBidAsSearcher(bid *auction.Bid, auctionEntryPoint *Account) (*auction.Bid, error) {
	bidHash := bid.GetHashTypedData(chainID, auctionEntryPoint.GetAddress())
	searcherSig, err := crypto.Sign(bidHash, a.privateKey[0])
	if err != nil {
		return nil, err
	}
	searcherSig[crypto.RecoveryIDOffset] += 27
	bid.SearcherSig = searcherSig
	return bid, nil
}

func (a *Account) signAuctionBidAsAuctioneer(auctionBid *auction.Bid, targetTxRaw []byte) (*auctionImpl.BidInput, error) {
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(auctionBid.SearcherSig), auctionBid.SearcherSig)
	hash := crypto.Keccak256([]byte(msg))
	auctioneerSig, err := crypto.Sign(hash, a.privateKey[0])
	if err != nil {
		return nil, err
	}

	auctioneerSig[crypto.RecoveryIDOffset] += 27
	bidInput := &auctionImpl.BidInput{
		TargetTxRaw:   targetTxRaw,
		TargetTxHash:  auctionBid.TargetTxHash,
		BlockNumber:   auctionBid.BlockNumber,
		Sender:        auctionBid.Sender,
		To:            auctionBid.To,
		Nonce:         auctionBid.Nonce,
		Bid:           hexutil.Big(*auctionBid.Bid),
		CallGasLimit:  auctionBid.CallGasLimit,
		Data:          auctionBid.Data,
		SearcherSig:   auctionBid.SearcherSig,
		AuctioneerSig: auctioneerSig,
	}
	return bidInput, nil
}

func toRlp(tx *types.Transaction) []byte {
	rlp, _ := rlp.EncodeToBytes(tx)
	return rlp
}

func isAuctionErr(err string) bool {
	return err == auction.ErrInitUnexpectedNil.Error() ||
		err == auction.ErrBlockNotFound.Error() ||
		err == auction.ErrInvalidBlockNumber.Error() ||
		err == auction.ErrInvalidSearcherSig.Error() ||
		err == auction.ErrInvalidAuctioneerSig.Error() ||
		err == auction.ErrNilChainId.Error() ||
		err == auction.ErrNilVerifyingContract.Error() ||
		err == auction.ErrInvalidTargetTxHash.Error() ||
		err == auction.ErrAuctionDisabled.Error() ||
		err == auction.ErrBidAlreadyExists.Error() ||
		err == auction.ErrBidSenderExists.Error() ||
		err == auction.ErrBidInvalidSearcherSig.Error() ||
		err == auction.ErrBidInvalidAuctioneerSig.Error() ||
		err == auction.ErrLowBid.Error() ||
		err == auction.ErrZeroBid.Error() ||
		err == auction.ErrBidPoolFull.Error() ||
		err == auction.ErrAuctionPaused.Error()
}

func ConcurrentTransactionSend(accs []*Account, maxConcurrency int, transactionSend func(*Account)) {
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.NumCPU() * 10 // default value
	}
	ch := make(chan int, maxConcurrency)
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
