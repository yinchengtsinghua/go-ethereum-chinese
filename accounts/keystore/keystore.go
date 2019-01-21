
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

//package keystore实现对secp256k1私钥的加密存储。
//
//根据Web3秘密存储规范，密钥存储为加密的JSON文件。
//有关详细信息，请参阅https://github.com/ethereum/wiki/wiki/web3-secret-storage-definition。
package keystore

import (
	"crypto/ecdsa"
	crand "crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
)

var (
	ErrLocked  = accounts.NewAuthNeededError("password or unlock")
	ErrNoMatch = errors.New("no key for given address or file")
	ErrDecrypt = errors.New("could not decrypt key with given passphrase")
)

//keystore type是keystore后端的反射类型。
var KeyStoreType = reflect.TypeOf(&KeyStore{})

//keystorescheme是协议方案的前缀帐户和钱包URL。
const KeyStoreScheme = "keystore"

//钱包刷新之间的最长时间（如果文件系统通知不起作用）。
const walletRefreshCycle = 3 * time.Second

//keystore管理磁盘上的密钥存储目录。
type KeyStore struct {
storage  keyStore                     //存储后端，可能是明文或加密的
cache    *accountCache                //文件系统存储上的内存帐户缓存
changes  chan struct{}                //从缓存接收更改通知的通道
unlocked map[common.Address]*unlocked //当前未锁定的帐户（解密的私钥）

wallets     []accounts.Wallet       //单个钥匙文件周围的钱包包装纸
updateFeed  event.Feed              //通知钱包添加/删除的事件源
updateScope event.SubscriptionScope //订阅范围跟踪当前实时侦听器
updating    bool                    //事件通知循环是否正在运行

	mu sync.RWMutex
}

type unlocked struct {
	*Key
	abort chan struct{}
}

//newkeystore为给定目录创建一个keystore。
func NewKeyStore(keydir string, scryptN, scryptP int) *KeyStore {
	keydir, _ = filepath.Abs(keydir)
	ks := &KeyStore{storage: &keyStorePassphrase{keydir, scryptN, scryptP, false}}
	ks.init(keydir)
	return ks
}

//newplaintextkeystore为给定目录创建一个keystore。
//已弃用：使用newkeystore。
func NewPlaintextKeyStore(keydir string) *KeyStore {
	keydir, _ = filepath.Abs(keydir)
	ks := &KeyStore{storage: &keyStorePlain{keydir}}
	ks.init(keydir)
	return ks
}

func (ks *KeyStore) init(keydir string) {
//锁定互斥体，因为帐户缓存可能使用事件回调
	ks.mu.Lock()
	defer ks.mu.Unlock()

//初始化一组未锁定的密钥和帐户缓存
	ks.unlocked = make(map[common.Address]*unlocked)
	ks.cache, ks.changes = newAccountCache(keydir)

//TODO:要使此终结器工作，必须没有引用
//对KS。AddressCache不保留引用，但未锁定的键会保留引用，
//因此，在所有定时解锁过期之前，终结器不会触发。
	runtime.SetFinalizer(ks, func(m *KeyStore) {
		m.cache.close()
	})
//从缓存创建钱包的初始列表
	accs := ks.cache.accounts()
	ks.wallets = make([]accounts.Wallet, len(accs))
	for i := 0; i < len(accs); i++ {
		ks.wallets[i] = &keystoreWallet{account: accs[i], keystore: ks}
	}
}

//钱包实现accounts.backend，从
//密钥存储目录。
func (ks *KeyStore) Wallets() []accounts.Wallet {
//确保钱包列表与帐户缓存同步
	ks.refreshWallets()

	ks.mu.RLock()
	defer ks.mu.RUnlock()

	cpy := make([]accounts.Wallet, len(ks.wallets))
	copy(cpy, ks.wallets)
	return cpy
}

//刷新钱包检索当前帐户列表，并基于此执行任何操作
//必要的钱包更新。
func (ks *KeyStore) refreshWallets() {
//检索当前帐户列表
	ks.mu.Lock()
	accs := ks.cache.accounts()

//将当前钱包列表转换为新列表
	wallets := make([]accounts.Wallet, 0, len(accs))
	events := []accounts.WalletEvent{}

	for _, account := range accs {
//把钱包放在下一个账户前
		for len(ks.wallets) > 0 && ks.wallets[0].URL().Cmp(account.URL) < 0 {
			events = append(events, accounts.WalletEvent{Wallet: ks.wallets[0], Kind: accounts.WalletDropped})
			ks.wallets = ks.wallets[1:]
		}
//如果没有更多的钱包或帐户在下一个之前，请包装新钱包
		if len(ks.wallets) == 0 || ks.wallets[0].URL().Cmp(account.URL) > 0 {
			wallet := &keystoreWallet{account: account, keystore: ks}

			events = append(events, accounts.WalletEvent{Wallet: wallet, Kind: accounts.WalletArrived})
			wallets = append(wallets, wallet)
			continue
		}
//如果账户与第一个钱包相同，请保留它
		if ks.wallets[0].Accounts()[0] == account {
			wallets = append(wallets, ks.wallets[0])
			ks.wallets = ks.wallets[1:]
			continue
		}
	}
//扔掉所有剩余的钱包，并设置新的一批
	for _, wallet := range ks.wallets {
		events = append(events, accounts.WalletEvent{Wallet: wallet, Kind: accounts.WalletDropped})
	}
	ks.wallets = wallets
	ks.mu.Unlock()

//启动所有钱包事件并返回
	for _, event := range events {
		ks.updateFeed.Send(event)
	}
}

//subscribe实现accounts.backend，创建对的异步订阅
//接收有关添加或删除密钥库钱包的通知。
func (ks *KeyStore) Subscribe(sink chan<- accounts.WalletEvent) event.Subscription {
//我们需要mutex来可靠地启动/停止更新循环
	ks.mu.Lock()
	defer ks.mu.Unlock()

//订阅调用方并跟踪订阅方计数
	sub := ks.updateScope.Track(ks.updateFeed.Subscribe(sink))

//订阅服务器需要一个活动的通知循环，启动它
	if !ks.updating {
		ks.updating = true
		go ks.updater()
	}
	return sub
}

//更新程序负责维护存储在
//密钥库，用于启动钱包添加/删除事件。它倾听
//来自基础帐户缓存的帐户更改事件，并且还定期
//强制手动刷新（仅适用于文件系统通知程序所在的系统
//没有运行）。
func (ks *KeyStore) updater() {
	for {
//等待帐户更新或刷新超时
		select {
		case <-ks.changes:
		case <-time.After(walletRefreshCycle):
		}
//运行钱包刷新程序
		ks.refreshWallets()

//如果我们所有的订户都离开了，请停止更新程序
		ks.mu.Lock()
		if ks.updateScope.Count() == 0 {
			ks.updating = false
			ks.mu.Unlock()
			return
		}
		ks.mu.Unlock()
	}
}

//hasAddress报告具有给定地址的密钥是否存在。
func (ks *KeyStore) HasAddress(addr common.Address) bool {
	return ks.cache.hasAddress(addr)
}

//帐户返回目录中存在的所有密钥文件。
func (ks *KeyStore) Accounts() []accounts.Account {
	return ks.cache.accounts()
}

//如果密码短语正确，则delete将删除与帐户匹配的密钥。
//
func (ks *KeyStore) Delete(a accounts.Account, passphrase string) error {
//解密密钥不是真正必要的，但我们确实需要
//不管怎样，检查密码然后把钥匙调零
//紧接着。
	a, key, err := ks.getDecryptedKey(a, passphrase)
	if key != nil {
		zeroKey(key.PrivateKey)
	}
	if err != nil {
		return err
	}
//这里的秩序至关重要。钥匙从
//文件消失后缓存，以便在
//between不会再将其插入缓存。
	err = os.Remove(a.URL.Path)
	if err == nil {
		ks.cache.delete(a)
		ks.refreshWallets()
	}
	return err
}

//signhash为给定哈希计算ECDSA签名。产生的
//签名采用[R S V]格式，其中V为0或1。
func (ks *KeyStore) SignHash(a accounts.Account, hash []byte) ([]byte, error) {
//查找要签名的密钥，如果找不到，则中止
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	unlockedKey, found := ks.unlocked[a.Address]
	if !found {
		return nil, ErrLocked
	}
//使用普通ECDSA操作对哈希进行签名
	return crypto.Sign(hash, unlockedKey.PrivateKey)
}

//signtx用请求的帐户对给定的事务进行签名。
func (ks *KeyStore) SignTx(a accounts.Account, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
//查找要签名的密钥，如果找不到，则中止
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	unlockedKey, found := ks.unlocked[a.Address]
	if !found {
		return nil, ErrLocked
	}
//根据链ID的存在，用EIP155或宅基地签名
	if chainID != nil {
		return types.SignTx(tx, types.NewEIP155Signer(chainID), unlockedKey.PrivateKey)
	}
	return types.SignTx(tx, types.HomesteadSigner{}, unlockedKey.PrivateKey)
}

//如果私钥与给定地址匹配，则signhashwithpassphrase将对哈希进行签名
//可以用给定的密码短语解密。生成的签名在
//[R_S_V]格式，其中V为0或1。
func (ks *KeyStore) SignHashWithPassphrase(a accounts.Account, passphrase string, hash []byte) (signature []byte, err error) {
	_, key, err := ks.getDecryptedKey(a, passphrase)
	if err != nil {
		return nil, err
	}
	defer zeroKey(key.PrivateKey)
	return crypto.Sign(hash, key.PrivateKey)
}

//如果私钥与
//给定的地址可以用给定的密码短语解密。
func (ks *KeyStore) SignTxWithPassphrase(a accounts.Account, passphrase string, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	_, key, err := ks.getDecryptedKey(a, passphrase)
	if err != nil {
		return nil, err
	}
	defer zeroKey(key.PrivateKey)

//根据链ID的存在，用EIP155或宅基地签名
	if chainID != nil {
		return types.SignTx(tx, types.NewEIP155Signer(chainID), key.PrivateKey)
	}
	return types.SignTx(tx, types.HomesteadSigner{}, key.PrivateKey)
}

//解锁无限期地解锁给定的帐户。
func (ks *KeyStore) Unlock(a accounts.Account, passphrase string) error {
	return ks.TimedUnlock(a, passphrase, 0)
}

//lock从内存中删除具有给定地址的私钥。
func (ks *KeyStore) Lock(addr common.Address) error {
	ks.mu.Lock()
	if unl, found := ks.unlocked[addr]; found {
		ks.mu.Unlock()
		ks.expire(addr, unl, time.Duration(0)*time.Nanosecond)
	} else {
		ks.mu.Unlock()
	}
	return nil
}

//timedunlock使用密码短语解锁给定的帐户。帐户
//在超时期间保持解锁状态。超时0将解锁帐户
//直到程序退出。帐户必须与唯一的密钥文件匹配。
//
//如果帐户地址在一段时间内已解锁，则TimedUnlock将扩展或
//缩短活动解锁超时。如果地址以前是解锁的
//无限期地超时不会改变。
func (ks *KeyStore) TimedUnlock(a accounts.Account, passphrase string, timeout time.Duration) error {
	a, key, err := ks.getDecryptedKey(a, passphrase)
	if err != nil {
		return err
	}

	ks.mu.Lock()
	defer ks.mu.Unlock()
	u, found := ks.unlocked[a.Address]
	if found {
		if u.abort == nil {
//地址被无限期地解锁，所以解锁
//超时会让人困惑。
			zeroKey(key.PrivateKey)
			return nil
		}
//终止过期的goroutine并在下面替换它。
		close(u.abort)
	}
	if timeout > 0 {
		u = &unlocked{Key: key, abort: make(chan struct{})}
		go ks.expire(a.Address, u, timeout)
	} else {
		u = &unlocked{Key: key}
	}
	ks.unlocked[a.Address] = u
	return nil
}

//find将给定帐户解析为密钥库中的唯一条目。
func (ks *KeyStore) Find(a accounts.Account) (accounts.Account, error) {
	ks.cache.maybeReload()
	ks.cache.mu.Lock()
	a, err := ks.cache.find(a)
	ks.cache.mu.Unlock()
	return a, err
}

func (ks *KeyStore) getDecryptedKey(a accounts.Account, auth string) (accounts.Account, *Key, error) {
	a, err := ks.Find(a)
	if err != nil {
		return a, nil, err
	}
	key, err := ks.storage.GetKey(a.Address, a.URL.Path, auth)
	return a, key, err
}

func (ks *KeyStore) expire(addr common.Address, u *unlocked, timeout time.Duration) {
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case <-u.abort:
//刚刚辞职
	case <-t.C:
		ks.mu.Lock()
//仅当它仍然是DropLater的同一个键实例时才删除
//与一起启动。我们可以用指针相等来检查
//因为每次键
//解锁。
		if ks.unlocked[addr] == u {
			zeroKey(u.PrivateKey)
			delete(ks.unlocked, addr)
		}
		ks.mu.Unlock()
	}
}

//newaccount生成一个新密钥并将其存储到密钥目录中，
//用密码短语加密。
func (ks *KeyStore) NewAccount(passphrase string) (accounts.Account, error) {
	_, account, err := storeNewKey(ks.storage, crand.Reader, passphrase)
	if err != nil {
		return accounts.Account{}, err
	}
//而是立即将帐户添加到缓存中
//而不是等待文件系统通知来接收它。
	ks.cache.add(account)
	ks.refreshWallets()
	return account, nil
}

//以json密钥的形式导出，并用newpassphrase加密。
func (ks *KeyStore) Export(a accounts.Account, passphrase, newPassphrase string) (keyJSON []byte, err error) {
	_, key, err := ks.getDecryptedKey(a, passphrase)
	if err != nil {
		return nil, err
	}
	var N, P int
	if store, ok := ks.storage.(*keyStorePassphrase); ok {
		N, P = store.scryptN, store.scryptP
	} else {
		N, P = StandardScryptN, StandardScryptP
	}
	return EncryptKey(key, newPassphrase, N, P)
}

//import将给定的加密JSON密钥存储到密钥目录中。
func (ks *KeyStore) Import(keyJSON []byte, passphrase, newPassphrase string) (accounts.Account, error) {
	key, err := DecryptKey(keyJSON, passphrase)
	if key != nil && key.PrivateKey != nil {
		defer zeroKey(key.PrivateKey)
	}
	if err != nil {
		return accounts.Account{}, err
	}
	return ks.importKey(key, newPassphrase)
}

//importecdsa将给定的密钥存储到密钥目录中，并用密码短语对其进行加密。
func (ks *KeyStore) ImportECDSA(priv *ecdsa.PrivateKey, passphrase string) (accounts.Account, error) {
	key := newKeyFromECDSA(priv)
	if ks.cache.hasAddress(key.Address) {
		return accounts.Account{}, fmt.Errorf("account already exists")
	}
	return ks.importKey(key, passphrase)
}

func (ks *KeyStore) importKey(key *Key, passphrase string) (accounts.Account, error) {
	a := accounts.Account{Address: key.Address, URL: accounts.URL{Scheme: KeyStoreScheme, Path: ks.storage.JoinPath(keyFileName(key.Address))}}
	if err := ks.storage.StoreKey(a.URL.Path, key, passphrase); err != nil {
		return accounts.Account{}, err
	}
	ks.cache.add(a)
	ks.refreshWallets()
	return a, nil
}

//更新更改现有帐户的密码。
func (ks *KeyStore) Update(a accounts.Account, passphrase, newPassphrase string) error {
	a, key, err := ks.getDecryptedKey(a, passphrase)
	if err != nil {
		return err
	}
	return ks.storage.StoreKey(a.URL.Path, key, newPassphrase)
}

//importpresalekey解密给定的以太坊预售钱包和商店
//密钥目录中的密钥文件。密钥文件使用相同的密码短语加密。
func (ks *KeyStore) ImportPreSaleKey(keyJSON []byte, passphrase string) (accounts.Account, error) {
	a, _, err := importPreSaleKey(ks.storage, keyJSON, passphrase)
	if err != nil {
		return a, err
	}
	ks.cache.add(a)
	ks.refreshWallets()
	return a, nil
}

//zerokey使内存中的私钥归零。
func zeroKey(k *ecdsa.PrivateKey) {
	b := k.D.Bits()
	for i := range b {
		b[i] = 0
	}
}
