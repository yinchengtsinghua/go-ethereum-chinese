
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//
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

//软件包usbwallet实现对USB硬件钱包的支持。
package usbwallet

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"sync"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/karalabe/hid"
)

//钱包健康检查之间检测USB拔出的最长时间。
const heartbeatCycle = time.Second

//在自派生尝试之间等待的最短时间，即使用户
//像疯了一样申请账户。
const selfDeriveThrottling = time.Second

//驱动程序定义特定于供应商的功能硬件钱包实例
//必须实施以允许在钱包生命周期管理中使用它们。
type driver interface {
//状态返回文本状态以帮助用户处于
//钱包。它还返回一个错误，指示钱包可能发生的任何故障。
//遇到。
	Status() (string, error)

//open初始化对钱包实例的访问。passphrase参数可以
//或者不可用于特定钱包实例的实现。
	Open(device io.ReadWriter, passphrase string) error

//关闭释放打开钱包实例持有的任何资源。
	Close() error

//Heartbeat对硬件钱包执行健全检查，以查看是否
//仍然在线且健康。
	Heartbeat() error

//派生向USB设备发送派生请求并返回以太坊
//地址位于该路径上。
	Derive(path accounts.DerivationPath) (common.Address, error)

//signtx将事务发送到USB设备并等待用户确认
//或者拒绝交易。
	SignTx(path accounts.DerivationPath, tx *types.Transaction, chainID *big.Int) (common.Address, *types.Transaction, error)
}

//Wallet代表所有USB硬件共享的通用功能
//防止重新实施相同复杂维护机制的钱包
//对于不同的供应商。
type wallet struct {
hub    *Hub          //USB集线器扫描
driver driver        //底层设备操作的硬件实现
url    *accounts.URL //唯一标识此钱包的文本URL

info   hid.DeviceInfo //已知的USB设备有关钱包的信息
device *hid.Device    //USB设备作为硬件钱包做广告

accounts []accounts.Account                         //固定在硬件钱包上的派生帐户列表
paths    map[common.Address]accounts.DerivationPath //签名操作的已知派生路径

deriveNextPath accounts.DerivationPath   //帐户自动发现的下一个派生路径
deriveNextAddr common.Address            //自动发现的下一个派生帐户地址
deriveChain    ethereum.ChainStateReader //区块链状态阅读器发现用过的账户
deriveReq      chan chan struct{}        //请求自派生的通道
deriveQuit     chan chan error           //终止自导数的通道

	healthQuit chan chan error

//锁定硬件钱包有点特别。因为硬件设备比较低
//执行时，与他们的任何通信可能需要
//时间。更糟糕的是，等待用户确认可能需要很长时间，
//但在这期间必须保持独家沟通。锁定整个钱包
//然而，在同一时间内，系统中任何不需要的部分都会停止运行。
//要进行通信，只需阅读一些状态（例如列出帐户）。
//
//因此，硬件钱包需要两个锁才能正常工作。国家
//锁可用于保护钱包软件侧的内部状态，其中
//不得在硬件通信期间独占。交流
//锁可以用来实现对设备本身的独占访问，这一个
//但是，应该允许“跳过”等待可能需要的操作
//使用该设备，但也可以不使用（例如，帐户自派生）。
//
//因为我们有两个锁，所以必须知道如何正确使用它们：
//-通信要求“device”不更改，因此获取
//commslock应该在有状态锁之后完成。
//-通信不得禁用对钱包状态的读取访问，因此
//只能将*read*锁保持为statelock。
commsLock chan struct{} //在不保持状态锁定的情况下，USB通信的互斥（buf=1）
stateLock sync.RWMutex  //保护对wallet结构字段的读写访问

log log.Logger //上下文记录器，用其ID标记基
}

//url实现accounts.wallet，返回usb硬件设备的url。
func (w *wallet) URL() accounts.URL {
return *w.url //不可变，不需要锁
}

//状态实现accounts.wallet，从
//底层特定于供应商的硬件钱包实现。
func (w *wallet) Status() (string, error) {
w.stateLock.RLock() //没有设备通信，状态锁就足够了
	defer w.stateLock.RUnlock()

	status, failure := w.driver.Status()
	if w.device == nil {
		return "Closed", failure
	}
	return status, failure
}

//打开implements accounts.wallet，尝试打开与
//硬件钱包。
func (w *wallet) Open(passphrase string) error {
w.stateLock.Lock() //状态锁已经足够了，因为此时还没有连接
	defer w.stateLock.Unlock()

//如果设备已打开一次，请拒绝重试
	if w.paths != nil {
		return accounts.ErrWalletAlreadyOpen
	}
//确保实际设备连接仅完成一次
	if w.device == nil {
		device, err := w.info.Open()
		if err != nil {
			return err
		}
		w.device = device
		w.commsLock = make(chan struct{}, 1)
w.commsLock <- struct{}{} //使能锁定
	}
//将设备初始化委托给基础驱动程序
	if err := w.driver.Open(w.device, passphrase); err != nil {
		return err
	}
//连接成功，开始生命周期管理
	w.paths = make(map[common.Address]accounts.DerivationPath)

	w.deriveReq = make(chan chan struct{})
	w.deriveQuit = make(chan chan error)
	w.healthQuit = make(chan chan error)

	go w.heartbeat()
	go w.selfDerive()

//通知任何收听钱包事件的人可以访问新设备
	go w.hub.updateFeed.Send(accounts.WalletEvent{Wallet: w, Kind: accounts.WalletOpened})

	return nil
}

//心跳是一个健康检查循环，用于USB钱包定期验证
//它们是否仍然存在，或者是否出现故障。
func (w *wallet) heartbeat() {
	w.log.Debug("USB wallet health-check started")
	defer w.log.Debug("USB wallet health-check stopped")

//执行心跳检查，直到终止或出错
	var (
		errc chan error
		err  error
	)
	for errc == nil && err == nil {
//等待直到请求终止或心跳周期到达
		select {
		case errc = <-w.healthQuit:
//请求终止
			continue
		case <-time.After(heartbeatCycle):
//心跳时间
		}
//执行微小的数据交换以查看响应
		w.stateLock.RLock()
		if w.device == nil {
//在等待锁时终止
			w.stateLock.RUnlock()
			continue
		}
<-w.commsLock //解析版本时不锁定状态
		err = w.driver.Heartbeat()
		w.commsLock <- struct{}{}
		w.stateLock.RUnlock()

		if err != nil {
w.stateLock.Lock() //锁定状态以将钱包撕下
			w.close()
			w.stateLock.Unlock()
		}
//忽略与硬件无关的错误
		err = nil
	}
//如果出现错误，请等待终止
	if err != nil {
		w.log.Debug("USB wallet health-check failed", "err", err)
		errc = <-w.healthQuit
	}
	errc <- err
}

//关闭机具帐户。钱包，关闭设备的USB连接。
func (w *wallet) Close() error {
//确保钱包已打开
	w.stateLock.RLock()
	hQuit, dQuit := w.healthQuit, w.deriveQuit
	w.stateLock.RUnlock()

//终止健康检查
	var herr error
	if hQuit != nil {
		errc := make(chan error)
		hQuit <- errc
herr = <-errc //保存供以后使用，我们*必须*关闭USB
	}
//终止自派生
	var derr error
	if dQuit != nil {
		errc := make(chan error)
		dQuit <- errc
derr = <-errc //保存供以后使用，我们*必须*关闭USB
	}
//终止设备连接
	w.stateLock.Lock()
	defer w.stateLock.Unlock()

	w.healthQuit = nil
	w.deriveQuit = nil
	w.deriveReq = nil

	if err := w.close(); err != nil {
		return err
	}
	if herr != nil {
		return herr
	}
	return derr
}

//Close是内部钱包闭合器，用于终止USB连接和
//将所有字段重置为默认值。
//
//注意，CLOSE假设状态锁被保持！
func (w *wallet) close() error {
//允许重复关闭，特别是在运行状况检查失败时
	if w.device == nil {
		return nil
	}
//关闭设备，清除所有内容，然后返回
	w.device.Close()
	w.device = nil

	w.accounts, w.paths = nil, nil
	w.driver.Close()

	return nil
}

//帐户实现帐户。钱包，返回固定到的帐户列表
//USB硬件钱包。如果启用了自派生，则帐户列表为
//根据当前链状态定期扩展。
func (w *wallet) Accounts() []accounts.Account {
//如果正在运行，尝试自派生
	reqc := make(chan struct{}, 1)
	select {
	case w.deriveReq <- reqc:
//已接受自派生请求，请稍候
		<-reqc
	default:
//脱机自派生、受限或忙碌、跳过
	}
//返回我们最终使用的任何帐户列表
	w.stateLock.RLock()
	defer w.stateLock.RUnlock()

	cpy := make([]accounts.Account, len(w.accounts))
	copy(cpy, w.accounts)
	return cpy
}

//selfderive是一个帐户派生循环，在请求时尝试查找
//新的非零账户。
func (w *wallet) selfDerive() {
	w.log.Debug("USB wallet self-derivation started")
	defer w.log.Debug("USB wallet self-derivation stopped")

//执行自派生直到终止或出错
	var (
		reqc chan struct{}
		errc chan error
		err  error
	)
	for errc == nil && err == nil {
//等待直到请求派生或终止
		select {
		case errc = <-w.deriveQuit:
//请求终止
			continue
		case reqc = <-w.deriveReq:
//已请求帐户发现
		}
//派生需要链和设备访问，如果不可用则跳过
		w.stateLock.RLock()
		if w.device == nil || w.deriveChain == nil {
			w.stateLock.RUnlock()
			reqc <- struct{}{}
			continue
		}
		select {
		case <-w.commsLock:
		default:
			w.stateLock.RUnlock()
			reqc <- struct{}{}
			continue
		}
//获取设备锁，派生下一批帐户
		var (
			accs  []accounts.Account
			paths []accounts.DerivationPath

			nextAddr = w.deriveNextAddr
			nextPath = w.deriveNextPath

			context = context.Background()
		)
		for empty := false; !empty; {
//检索下一个派生的以太坊帐户
			if nextAddr == (common.Address{}) {
				if nextAddr, err = w.driver.Derive(nextPath); err != nil {
					w.log.Warn("USB wallet account derivation failed", "err", err)
					break
				}
			}
//对照当前链状态检查帐户状态
			var (
				balance *big.Int
				nonce   uint64
			)
			balance, err = w.deriveChain.BalanceAt(context, nextAddr, nil)
			if err != nil {
				w.log.Warn("USB wallet balance retrieval failed", "err", err)
				break
			}
			nonce, err = w.deriveChain.NonceAt(context, nextAddr, nil)
			if err != nil {
				w.log.Warn("USB wallet nonce retrieval failed", "err", err)
				break
			}
//如果下一个帐户为空，请停止自派生，但仍要添加它。
			if balance.Sign() == 0 && nonce == 0 {
				empty = true
			}
//我们刚刚自己创建了一个新帐户，开始在本地跟踪它
			path := make(accounts.DerivationPath, len(nextPath))
			copy(path[:], nextPath[:])
			paths = append(paths, path)

			account := accounts.Account{
				Address: nextAddr,
				URL:     accounts.URL{Scheme: w.url.Scheme, Path: fmt.Sprintf("%s/%s", w.url.Path, path)},
			}
			accs = append(accs, account)

//为新帐户（或以前为空的帐户）向用户显示日志消息
			if _, known := w.paths[nextAddr]; !known || (!empty && nextAddr == w.deriveNextAddr) {
				w.log.Info("USB wallet discovered new account", "address", nextAddr, "path", path, "balance", balance, "nonce", nonce)
			}
//获取下一个潜在帐户
			if !empty {
				nextAddr = common.Address{}
				nextPath[len(nextPath)-1]++
			}
		}
//自派生完成，释放装置锁
		w.commsLock <- struct{}{}
		w.stateLock.RUnlock()

//插入成功派生的任何帐户
		w.stateLock.Lock()
		for i := 0; i < len(accs); i++ {
			if _, ok := w.paths[accs[i].Address]; !ok {
				w.accounts = append(w.accounts, accs[i])
				w.paths[accs[i].Address] = paths[i]
			}
		}
//向前移动自派生
//TODO（卡拉贝拉）：不要覆盖wallet.self派生的更改
		w.deriveNextAddr = nextAddr
		w.deriveNextPath = nextPath
		w.stateLock.Unlock()

//一段时间后通知用户终止和循环（以避免损坏）
		reqc <- struct{}{}
		if err == nil {
			select {
			case errc = <-w.deriveQuit:
//请求终止，中止
			case <-time.After(selfDeriveThrottling):
//等得够久，愿意自己再推导一次
			}
		}
	}
//如果出现错误，请等待终止
	if err != nil {
		w.log.Debug("USB wallet self-derivation failed", "err", err)
		errc = <-w.deriveQuit
	}
	errc <- err
}

//包含implements accounts.wallet，返回特定帐户是否为
//或未固定到此钱包实例中。尽管我们可以尝试解决
//取消固定帐户，这将是一个不可忽略的硬件操作。
func (w *wallet) Contains(account accounts.Account) bool {
	w.stateLock.RLock()
	defer w.stateLock.RUnlock()

	_, exists := w.paths[account.Address]
	return exists
}

//派生实现accounts.wallet，在特定的
//派生路径。如果pin设置为true，则帐户将添加到列表中
//个跟踪帐户。
func (w *wallet) Derive(path accounts.DerivationPath, pin bool) (accounts.Account, error) {
//如果成功，尝试派生实际帐户并更新其URL
w.stateLock.RLock() //避免设备在派生过程中消失

	if w.device == nil {
		w.stateLock.RUnlock()
		return accounts.Account{}, accounts.ErrWalletClosed
	}
<-w.commsLock //避免并行硬件访问
	address, err := w.driver.Derive(path)
	w.commsLock <- struct{}{}

	w.stateLock.RUnlock()

//如果发生错误或未请求固定，请返回
	if err != nil {
		return accounts.Account{}, err
	}
	account := accounts.Account{
		Address: address,
		URL:     accounts.URL{Scheme: w.url.Scheme, Path: fmt.Sprintf("%s/%s", w.url.Path, path)},
	}
	if !pin {
		return account, nil
	}
//固定需要修改状态
	w.stateLock.Lock()
	defer w.stateLock.Unlock()

	if _, ok := w.paths[address]; !ok {
		w.accounts = append(w.accounts, account)
		w.paths[address] = path
	}
	return account, nil
}

//selfderive实现accounts.wallet，尝试发现
//用户以前使用过（基于链状态），但他/她没有使用过
//手动明确地固定到钱包。为了避免链头监控，请自行
//派生仅在帐户列表期间运行（甚至在随后被限制）。
func (w *wallet) SelfDerive(base accounts.DerivationPath, chain ethereum.ChainStateReader) {
	w.stateLock.Lock()
	defer w.stateLock.Unlock()

	w.deriveNextPath = make(accounts.DerivationPath, len(base))
	copy(w.deriveNextPath[:], base[:])

	w.deriveNextAddr = common.Address{}
	w.deriveChain = chain
}

//signhash实现accounts.wallet，但是签名任意数据不是
//支持硬件钱包，因此此方法将始终返回错误。
func (w *wallet) SignHash(account accounts.Account, hash []byte) ([]byte, error) {
	return nil, accounts.ErrNotSupported
}

//signtx实现accounts.wallet。它将交易发送到分类帐
//要求用户确认的钱包。它返回已签名的
//如果用户拒绝该事务，则为事务或失败。
//
//注意，如果运行在Ledger钱包上的以太坊应用程序的版本是
//太旧，无法签署EIP-155交易，但要求这样做，还是有错误
//
func (w *wallet) SignTx(account accounts.Account, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
w.stateLock.RLock() //通信有自己的互斥，这是用于状态字段
	defer w.stateLock.RUnlock()

//如果钱包关闭，中止
	if w.device == nil {
		return nil, accounts.ErrWalletClosed
	}
//确保请求的帐户包含在
	path, ok := w.paths[account.Address]
	if !ok {
		return nil, accounts.ErrUnknownAccount
	}
//收集的所有信息和元数据均已签出，请求签名
	<-w.commsLock
	defer func() { w.commsLock <- struct{}{} }()

//在等待用户确认时，确保设备没有拧紧。
//TODO（karalabe）：如果热插拔落在Windows上，则移除。
	w.hub.commsLock.Lock()
	w.hub.commsPend++
	w.hub.commsLock.Unlock()

	defer func() {
		w.hub.commsLock.Lock()
		w.hub.commsPend--
		w.hub.commsLock.Unlock()
	}()
//签署事务并验证发送方以避免硬件故障意外
	sender, signed, err := w.driver.SignTx(path, tx, chainID)
	if err != nil {
		return nil, err
	}
	if sender != account.Address {
		return nil, fmt.Errorf("signer mismatch: expected %s, got %s", account.Address.Hex(), sender.Hex())
	}
	return signed, nil
}

//signhashwithpassphrase实现accounts.wallet，但是任意签名
//分类帐钱包不支持数据，因此此方法将始终返回
//一个错误。
func (w *wallet) SignHashWithPassphrase(account accounts.Account, passphrase string, hash []byte) ([]byte, error) {
	return w.SignHash(account, hash)
}

//signtxwithpassphrase实现accounts.wallet，尝试对给定的
//使用密码短语作为额外身份验证的给定帐户的事务。
//由于USB钱包不依赖密码，因此这些密码会被静默忽略。
func (w *wallet) SignTxWithPassphrase(account accounts.Account, passphrase string, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	return w.SignTx(account, tx, chainID)
}
