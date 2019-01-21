
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//Go-Ethereum库是免费软件：您可以重新分发它和/或修改
//根据GNU发布的较低通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊图书馆的发行目的是希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

//包裹支票簿包裹“支票簿”以太坊智能合约。
//
//此包中的函数允许使用支票簿
//在以太网上签发、接收、验证支票；（自动）在以太网上兑现支票
//以及（自动）将以太存入支票簿合同。
package chequebook

//go:生成abigen--sol contract/checkbook.sol--exc contract/凡人.sol:凡人，contract/owned.sol:owned--pkg contract--out contract/checkbook.go
//go：生成go run./gencode.go

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/contracts/chequebook/contract"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/swarm/services/swap/swap"
)

//托多（泽利格）：观察同龄人的偿付能力，并通知跳票
//TODO（Zelig）：通过签核启用支票付款

//有些功能需要与区块链交互：
//*设置对等支票簿的当前余额
//*发送交易以兑现支票
//*将乙醚存入支票簿
//*观察进入的乙醚

var (
gasToCash = uint64(2000000) //使用支票簿的现金交易的天然气成本
//
)

//后端包装支票簿操作所需的所有方法。
type Backend interface {
	bind.ContractBackend
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	BalanceAt(ctx context.Context, address common.Address, blockNum *big.Int) (*big.Int, error)
}

//支票代表对单一受益人的付款承诺。
type Cheque struct {
Contract    common.Address //支票簿地址，避免交叉合同提交
	Beneficiary common.Address
Amount      *big.Int //所有已发送资金的累计金额
Sig         []byte   //签字（KECCAK256（合同、受益人、金额）、prvkey）
}

func (self *Cheque) String() string {
	return fmt.Sprintf("contract: %s, beneficiary: %s, amount: %v, signature: %x", self.Contract.Hex(), self.Beneficiary.Hex(), self.Amount, self.Sig)
}

type Params struct {
	ContractCode, ContractAbi string
}

var ContractParams = &Params{contract.ChequebookBin, contract.ChequebookABI}

//支票簿可以创建并签署从单个合同到多个受益人的支票。
//它是对等小额支付的传出支付处理程序。
type Chequebook struct {
path     string                      //支票簿文件路径
prvKey   *ecdsa.PrivateKey           //用于签署支票的私钥
lock     sync.Mutex                  //
backend  Backend                     //块链API
quit     chan bool                   //关闭时会导致自动报告停止
owner    common.Address              //所有者地址（从pubkey派生）
contract *contract.Chequebook        //非本征结合
session  *contract.ChequebookSession //Abigen与Tx OPT结合

//保留字段
balance      *big.Int                    //未与区块链同步
contractAddr common.Address              //合同地址
sent         map[common.Address]*big.Int //受益人计数

txhash    string   //上次存款的Tx哈希Tx
threshold *big.Int //如果不是零，则触发自动报告的阈值
buffer    *big.Int //保持平衡的缓冲器，用于叉保护

log log.Logger //嵌入合同地址的上下文记录器
}

func (self *Chequebook) String() string {
	return fmt.Sprintf("contract: %s, owner: %s, balance: %v, signer: %x", self.contractAddr.Hex(), self.owner.Hex(), self.balance, self.prvKey.PublicKey)
}

//新支票簿创建新支票簿。
func NewChequebook(path string, contractAddr common.Address, prvKey *ecdsa.PrivateKey, backend Backend) (self *Chequebook, err error) {
	balance := new(big.Int)
	sent := make(map[common.Address]*big.Int)

	chbook, err := contract.NewChequebook(contractAddr, backend)
	if err != nil {
		return nil, err
	}
	transactOpts := bind.NewKeyedTransactor(prvKey)
	session := &contract.ChequebookSession{
		Contract:     chbook,
		TransactOpts: *transactOpts,
	}

	self = &Chequebook{
		prvKey:       prvKey,
		balance:      balance,
		contractAddr: contractAddr,
		sent:         sent,
		path:         path,
		backend:      backend,
		owner:        transactOpts.From,
		contract:     chbook,
		session:      session,
		log:          log.New("contract", contractAddr),
	}

	if (contractAddr != common.Address{}) {
		self.setBalanceFromBlockChain()
		self.log.Trace("New chequebook initialised", "owner", self.owner, "balance", self.balance)
	}
	return
}

func (self *Chequebook) setBalanceFromBlockChain() {
	balance, err := self.backend.BalanceAt(context.TODO(), self.contractAddr, nil)
	if err != nil {
		log.Error("Failed to retrieve chequebook balance", "err", err)
	} else {
		self.balance.Set(balance)
	}
}

//加载支票簿从磁盘（文件路径）加载支票簿。
func LoadChequebook(path string, prvKey *ecdsa.PrivateKey, backend Backend, checkBalance bool) (self *Chequebook, err error) {
	var data []byte
	data, err = ioutil.ReadFile(path)
	if err != nil {
		return
	}
	self, _ = NewChequebook(path, common.Address{}, prvKey, backend)

	err = json.Unmarshal(data, self)
	if err != nil {
		return nil, err
	}
	if checkBalance {
		self.setBalanceFromBlockChain()
	}
	log.Trace("Loaded chequebook from disk", "path", path)

	return
}

//支票簿文件是支票簿的JSON表示。
type chequebookFile struct {
	Balance  string
	Contract string
	Owner    string
	Sent     map[string]string
}

//取消对支票簿的反序列化。
func (self *Chequebook) UnmarshalJSON(data []byte) error {
	var file chequebookFile
	err := json.Unmarshal(data, &file)
	if err != nil {
		return err
	}
	_, ok := self.balance.SetString(file.Balance, 10)
	if !ok {
		return fmt.Errorf("cumulative amount sent: unable to convert string to big integer: %v", file.Balance)
	}
	self.contractAddr = common.HexToAddress(file.Contract)
	for addr, sent := range file.Sent {
		self.sent[common.HexToAddress(addr)], ok = new(big.Int).SetString(sent, 10)
		if !ok {
			return fmt.Errorf("beneficiary %v cumulative amount sent: unable to convert string to big integer: %v", addr, sent)
		}
	}
	return nil
}

//Marshaljson将支票簿序列化。
func (self *Chequebook) MarshalJSON() ([]byte, error) {
	var file = &chequebookFile{
		Balance:  self.balance.String(),
		Contract: self.contractAddr.Hex(),
		Owner:    self.owner.Hex(),
		Sent:     make(map[string]string),
	}
	for addr, sent := range self.sent {
		file.Sent[addr.Hex()] = sent.String()
	}
	return json.Marshal(file)
}

//保存将支票簿保存在磁盘上，记住余额、合同地址和
//为每个受益人发送的资金的累计金额。
func (self *Chequebook) Save() (err error) {
	data, err := json.MarshalIndent(self, "", " ")
	if err != nil {
		return err
	}
	self.log.Trace("Saving chequebook to disk", self.path)

	return ioutil.WriteFile(self.path, data, os.ModePerm)
}

//Stop退出自动报告Go例程以终止
func (self *Chequebook) Stop() {
	defer self.lock.Unlock()
	self.lock.Lock()
	if self.quit != nil {
		close(self.quit)
		self.quit = nil
	}
}

//发行创建由支票簿所有者的私钥签名的支票。这个
//签字人承诺合同（他们拥有的），受益人和金额。
func (self *Chequebook) Issue(beneficiary common.Address, amount *big.Int) (ch *Cheque, err error) {
	defer self.lock.Unlock()
	self.lock.Lock()

	if amount.Sign() <= 0 {
		return nil, fmt.Errorf("amount must be greater than zero (%v)", amount)
	}
	if self.balance.Cmp(amount) < 0 {
		err = fmt.Errorf("insufficient funds to issue cheque for amount: %v. balance: %v", amount, self.balance)
	} else {
		var sig []byte
		sent, found := self.sent[beneficiary]
		if !found {
			sent = new(big.Int)
			self.sent[beneficiary] = sent
		}
		sum := new(big.Int).Set(sent)
		sum.Add(sum, amount)

		sig, err = crypto.Sign(sigHash(self.contractAddr, beneficiary, sum), self.prvKey)
		if err == nil {
			ch = &Cheque{
				Contract:    self.contractAddr,
				Beneficiary: beneficiary,
				Amount:      sum,
				Sig:         sig,
			}
			sent.Set(sum)
self.balance.Sub(self.balance, amount) //从余额中减去金额
		}
	}

//如果设置了阈值且余额小于阈值，则自动存款
//请注意，即使签发支票失败，也会调用此函数。
//所以我们重新尝试存放
	if self.threshold != nil {
		if self.balance.Cmp(self.threshold) < 0 {
			send := new(big.Int).Sub(self.buffer, self.balance)
			self.deposit(send)
		}
	}

	return
}

//现金是兑现支票的一种方便方法。
func (self *Chequebook) Cash(ch *Cheque) (txhash string, err error) {
	return ch.Cash(self.session)
}

//签署资料：合同地址、受益人、累计发送资金金额
func sigHash(contract, beneficiary common.Address, sum *big.Int) []byte {
	bigamount := sum.Bytes()
	if len(bigamount) > 32 {
		return nil
	}
	var amount32 [32]byte
	copy(amount32[32-len(bigamount):32], bigamount)
	input := append(contract.Bytes(), beneficiary.Bytes()...)
	input = append(input, amount32[:]...)
	return crypto.Keccak256(input)
}

//余额返回支票簿的当前余额。
func (self *Chequebook) Balance() *big.Int {
	defer self.lock.Unlock()
	self.lock.Lock()
	return new(big.Int).Set(self.balance)
}

//所有者返回支票簿的所有者帐户。
func (self *Chequebook) Owner() common.Address {
	return self.owner
}

//地址返回支票簿的链上合同地址。
func (self *Chequebook) Address() common.Address {
	return self.contractAddr
}

//把钱存入支票簿帐户。
func (self *Chequebook) Deposit(amount *big.Int) (string, error) {
	defer self.lock.Unlock()
	self.lock.Lock()
	return self.deposit(amount)
}

//存款金额到支票簿帐户。
//调用方必须保持self.lock。
func (self *Chequebook) deposit(amount *big.Int) (string, error) {
//因为这里的金额是可变的，所以我们不使用会话
	depositTransactor := bind.NewKeyedTransactor(self.prvKey)
	depositTransactor.Value = amount
	chbookRaw := &contract.ChequebookRaw{Contract: self.contract}
	tx, err := chbookRaw.Transfer(depositTransactor)
	if err != nil {
		self.log.Warn("Failed to fund chequebook", "amount", amount, "balance", self.balance, "target", self.buffer, "err", err)
		return "", err
	}
//假设交易实际成功，我们立即添加余额。
	self.balance.Add(self.balance, amount)
	self.log.Trace("Deposited funds to chequebook", "amount", amount, "balance", self.balance, "target", self.buffer)
	return tx.Hash().Hex(), nil
}

//autodeposit（re）设置触发向
//支票簿。如果阈值不小于缓冲区，则需要设置合同后端，然后
//每一张新支票都会触发存款。
func (self *Chequebook) AutoDeposit(interval time.Duration, threshold, buffer *big.Int) {
	defer self.lock.Unlock()
	self.lock.Lock()
	self.threshold = threshold
	self.buffer = buffer
	self.autoDeposit(interval)
}

//自动报告启动一个定期向支票簿发送资金的goroutine
//如果checkbook.quit已关闭，则合同调用方将持有执行例程终止的锁。
func (self *Chequebook) autoDeposit(interval time.Duration) {
	if self.quit != nil {
		close(self.quit)
		self.quit = nil
	}
//如果阈值大于等于每次签发支票后自动保存余额
	if interval == time.Duration(0) || self.threshold != nil && self.buffer != nil && self.threshold.Cmp(self.buffer) >= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	self.quit = make(chan bool)
	quit := self.quit

	go func() {
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				self.lock.Lock()
				if self.balance.Cmp(self.buffer) < 0 {
					amount := new(big.Int).Sub(self.buffer, self.balance)
					txhash, err := self.deposit(amount)
					if err == nil {
						self.txhash = txhash
					}
				}
				self.lock.Unlock()
			}
		}
	}()
}

//发件箱可以向单个受益人签发来自单个合同的支票。
type Outbox struct {
	chequeBook  *Chequebook
	beneficiary common.Address
}

//新发件箱创建发件箱。
func NewOutbox(chbook *Chequebook, beneficiary common.Address) *Outbox {
	return &Outbox{chbook, beneficiary}
}

//发行创建支票。
func (self *Outbox) Issue(amount *big.Int) (swap.Promise, error) {
	return self.chequeBook.Issue(self.beneficiary, amount)
}

//自动存单可以在基础支票簿上自动存款。
func (self *Outbox) AutoDeposit(interval time.Duration, threshold, buffer *big.Int) {
	self.chequeBook.AutoDeposit(interval, threshold, buffer)
}

//stop帮助满足swap.outpayment接口。
func (self *Outbox) Stop() {}

//字符串实现fmt.stringer。
func (self *Outbox) String() string {
	return fmt.Sprintf("chequebook: %v, beneficiary: %s, balance: %v", self.chequeBook.Address().Hex(), self.beneficiary.Hex(), self.chequeBook.Balance())
}

//收件箱可以将单个合同中的支票存入、验证和兑现为单个合同中的支票。
//受益人。它是对等小额支付的传入支付处理程序。
type Inbox struct {
	lock        sync.Mutex
contract    common.Address              //同行支票簿合同
beneficiary common.Address              //本地对等机的接收地址
sender      common.Address              //要从中发送兑现发送信息的本地对等方地址
signer      *ecdsa.PublicKey            //对等方的公钥
txhash      string                      //上次兑现的Tx哈希Tx
session     *contract.ChequebookSession //ABI与TX OPT的后端合同
quit        chan bool                   //当关闭时，自动灰烬停止
maxUncashed *big.Int                    //触发自动清除的阈值
cashed      *big.Int                    //累计兑现金额
cheque      *Cheque                     //最后一张支票，如果没有收到，则为零。
log         log.Logger                  //嵌入合同地址的上下文记录器
}

//newinbox创建一个收件箱。未持久化收件箱，将更新累积和
//收到第一张支票时从区块链获取。
func NewInbox(prvKey *ecdsa.PrivateKey, contractAddr, beneficiary common.Address, signer *ecdsa.PublicKey, abigen bind.ContractBackend) (self *Inbox, err error) {
	if signer == nil {
		return nil, fmt.Errorf("signer is null")
	}
	chbook, err := contract.NewChequebook(contractAddr, abigen)
	if err != nil {
		return nil, err
	}
	transactOpts := bind.NewKeyedTransactor(prvKey)
	transactOpts.GasLimit = gasToCash
	session := &contract.ChequebookSession{
		Contract:     chbook,
		TransactOpts: *transactOpts,
	}
	sender := transactOpts.From

	self = &Inbox{
		contract:    contractAddr,
		beneficiary: beneficiary,
		sender:      sender,
		signer:      signer,
		session:     session,
		cashed:      new(big.Int).Set(common.Big0),
		log:         log.New("contract", contractAddr),
	}
	self.log.Trace("New chequebook inbox initialized", "beneficiary", self.beneficiary, "signer", hexutil.Bytes(crypto.FromECDSAPub(signer)))
	return
}

func (self *Inbox) String() string {
	return fmt.Sprintf("chequebook: %v, beneficiary: %s, balance: %v", self.contract.Hex(), self.beneficiary.Hex(), self.cheque.Amount)
}

//停止退出自动清除引擎。
func (self *Inbox) Stop() {
	defer self.lock.Unlock()
	self.lock.Lock()
	if self.quit != nil {
		close(self.quit)
		self.quit = nil
	}
}

//现金试图兑现当前支票。
func (self *Inbox) Cash() (txhash string, err error) {
	if self.cheque != nil {
		txhash, err = self.cheque.Cash(self.session)
		self.log.Trace("Cashing in chequebook cheque", "amount", self.cheque.Amount, "beneficiary", self.beneficiary)
		self.cashed = self.cheque.Amount
	}
	return
}

//autocash（re）设置触发上次未兑现兑现的最大时间和金额。
//如果maxuncashed设置为0，则在收到时自动清除。
func (self *Inbox) AutoCash(cashInterval time.Duration, maxUncashed *big.Int) {
	defer self.lock.Unlock()
	self.lock.Lock()
	self.maxUncashed = maxUncashed
	self.autoCash(cashInterval)
}

//autocash启动一个循环，周期性地清除最后一张支票。
//如果对等方是可信的。清理时间可以是24小时或一周。
//调用方必须保持self.lock。
func (self *Inbox) autoCash(cashInterval time.Duration) {
	if self.quit != nil {
		close(self.quit)
		self.quit = nil
	}
//如果maxuncashed设置为0，则在接收时自动清除
	if cashInterval == time.Duration(0) || self.maxUncashed != nil && self.maxUncashed.Sign() == 0 {
		return
	}

	ticker := time.NewTicker(cashInterval)
	self.quit = make(chan bool)
	quit := self.quit

	go func() {
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				self.lock.Lock()
				if self.cheque != nil && self.cheque.Amount.Cmp(self.cashed) != 0 {
					txhash, err := self.Cash()
					if err == nil {
						self.txhash = txhash
					}
				}
				self.lock.Unlock()
			}
		}
	}()
}

//Receive被调用将最新的支票存入接收的收件箱。
//给出的承诺必须是一张*支票。
func (self *Inbox) Receive(promise swap.Promise) (*big.Int, error) {
	ch := promise.(*Cheque)

	defer self.lock.Unlock()
	self.lock.Lock()

	var sum *big.Int
	if self.cheque == nil {
//一旦收到支票，金额将与区块链核对。
		tally, err := self.session.Sent(self.beneficiary)
		if err != nil {
			return nil, fmt.Errorf("inbox: error calling backend to set amount: %v", err)
		}
		sum = tally
	} else {
		sum = self.cheque.Amount
	}

	amount, err := ch.Verify(self.signer, self.contract, self.beneficiary, sum)
	var uncashed *big.Int
	if err == nil {
		self.cheque = ch

		if self.maxUncashed != nil {
			uncashed = new(big.Int).Sub(ch.Amount, self.cashed)
			if self.maxUncashed.Cmp(uncashed) < 0 {
				self.Cash()
			}
		}
		self.log.Trace("Received cheque in chequebook inbox", "amount", amount, "uncashed", uncashed)
	}

	return amount, err
}

//核实支票的签字人、合同人、受益人、金额、有效签字。
func (self *Cheque) Verify(signerKey *ecdsa.PublicKey, contract, beneficiary common.Address, sum *big.Int) (*big.Int, error) {
	log.Trace("Verifying chequebook cheque", "cheque", self, "sum", sum)
	if sum == nil {
		return nil, fmt.Errorf("invalid amount")
	}

	if self.Beneficiary != beneficiary {
		return nil, fmt.Errorf("beneficiary mismatch: %v != %v", self.Beneficiary.Hex(), beneficiary.Hex())
	}
	if self.Contract != contract {
		return nil, fmt.Errorf("contract mismatch: %v != %v", self.Contract.Hex(), contract.Hex())
	}

	amount := new(big.Int).Set(self.Amount)
	if sum != nil {
		amount.Sub(amount, sum)
		if amount.Sign() <= 0 {
			return nil, fmt.Errorf("incorrect amount: %v <= 0", amount)
		}
	}

	pubKey, err := crypto.SigToPub(sigHash(self.Contract, beneficiary, self.Amount), self.Sig)
	if err != nil {
		return nil, fmt.Errorf("invalid signature: %v", err)
	}
	if !bytes.Equal(crypto.FromECDSAPub(pubKey), crypto.FromECDSAPub(signerKey)) {
		return nil, fmt.Errorf("signer mismatch: %x != %x", crypto.FromECDSAPub(pubKey), crypto.FromECDSAPub(signerKey))
	}
	return amount, nil
}

//签名的V/R/S表示
func sig2vrs(sig []byte) (v byte, r, s [32]byte) {
	v = sig[64] + 27
	copy(r[:], sig[:32])
	copy(s[:], sig[32:64])
	return
}

//现金通过发送以太坊交易来兑现支票。
func (self *Cheque) Cash(session *contract.ChequebookSession) (string, error) {
	v, r, s := sig2vrs(self.Sig)
	tx, err := session.Cash(self.Beneficiary, self.Amount, v, r, s)
	if err != nil {
		return "", err
	}
	return tx.Hash().Hex(), nil
}

//validatecode检查地址处的链上代码是否与预期的支票簿匹配
//合同代码。这用于检测自杀支票簿。
func ValidateCode(ctx context.Context, b Backend, address common.Address) (ok bool, err error) {
	code, err := b.CodeAt(ctx, address, nil)
	if err != nil {
		return false, err
	}
	return bytes.Equal(code, common.FromHex(contract.ContractDeployedCode)), nil
}
