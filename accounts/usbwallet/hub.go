
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

package usbwallet

import (
	"errors"
	"runtime"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/karalabe/hid"
)

//Ledgerscheme是协议方案的前缀帐户和钱包URL。
const LedgerScheme = "ledger"

//Trezorscheme是协议方案的前缀帐户和钱包URL。
const TrezorScheme = "trezor"

//刷新周期是钱包刷新之间的最长时间（如果是USB热插拔
//通知不起作用）。
const refreshCycle = time.Second

//RefreshThrottling是钱包刷新之间避免USB的最短时间间隔。
//蹂躏
const refreshThrottling = 500 * time.Millisecond

//Hub是一个帐户。后端可以查找和处理通用的USB硬件钱包。
type Hub struct {
scheme     string                  //协议方案前缀帐户和钱包URL。
vendorID   uint16                  //用于设备发现的USB供应商标识符
productIDs []uint16                //用于设备发现的USB产品标识符
usageID    uint16                  //用于MacOS设备发现的USB使用页标识符
endpointID int                     //用于非MacOS设备发现的USB端点标识符
makeDriver func(log.Logger) driver //构造供应商特定驱动程序的工厂方法

refreshed   time.Time               //上次刷新钱包列表时的时间实例
wallets     []accounts.Wallet       //当前跟踪的USB钱包设备列表
updateFeed  event.Feed              //通知钱包添加/删除的事件源
updateScope event.SubscriptionScope //订阅范围跟踪当前实时侦听器
updating    bool                    //事件通知循环是否正在运行

	quit chan chan error

stateLock sync.RWMutex //保护轮毂内部不受滚道的影响

//TODO（karalabe）：如果热插拔落在Windows上，则移除。
commsPend int        //阻止枚举的操作数
commsLock sync.Mutex //保护挂起计数器和枚举的锁
}

//newledgerhub为分类帐设备创建了一个新的硬件钱包管理器。
func NewLedgerHub() (*Hub, error) {
 /*urn newhub（Ledgerscheme，0x2c97，[]uint16 0x0000/*Ledger Blue*/，0x001/*Ledger Nano S*/，0xffa0，0，NewledgerDriver）
}

//newtrezorhub为trezor设备创建新的硬件钱包管理器。
func newtrezorhub（）（*hub，错误）
 返回newhub（trezorscheme，0x534c，[]uint16 0x001/*trezor 1*/}, 0xff00, 0, newTrezorDriver)

}

//NewHub为通用USB设备创建了一个新的硬件钱包管理器。
func newHub(scheme string, vendorID uint16, productIDs []uint16, usageID uint16, endpointID int, makeDriver func(log.Logger) driver) (*Hub, error) {
	if !hid.Supported() {
		return nil, errors.New("unsupported platform")
	}
	hub := &Hub{
		scheme:     scheme,
		vendorID:   vendorID,
		productIDs: productIDs,
		usageID:    usageID,
		endpointID: endpointID,
		makeDriver: makeDriver,
		quit:       make(chan chan error),
	}
	hub.refreshWallets()
	return hub, nil
}

//钱包实现帐户。后端，返回当前跟踪的所有USB
//看起来像是硬件钱包的设备。
func (hub *Hub) Wallets() []accounts.Wallet {
//确保钱包列表是最新的
	hub.refreshWallets()

	hub.stateLock.RLock()
	defer hub.stateLock.RUnlock()

	cpy := make([]accounts.Wallet, len(hub.wallets))
	copy(cpy, hub.wallets)
	return cpy
}

//刷新钱包扫描连接到机器的USB设备并更新
//基于找到的设备的钱包列表。
func (hub *Hub) refreshWallets() {
//不要像疯了一样扫描USB，用户会在一个循环中取出钱包。
	hub.stateLock.RLock()
	elapsed := time.Since(hub.refreshed)
	hub.stateLock.RUnlock()

	if elapsed < refreshThrottling {
		return
	}
//检索USB钱包设备的当前列表
	var devices []hid.DeviceInfo

	if runtime.GOOS == "linux" {
//Linux上的hidapi在枚举期间打开设备以检索一些信息，
//如果正在等待用户确认，则破坏分类帐协议。这个
//在分类帐上确认了错误，但不会在旧设备上修复，所以我们
//需要自己防止并发通信。更优雅的解决方案是
//放弃枚举以支持热插拔事件，但这还不起作用
//在Windows上，所以如果我们无论如何都需要破解它，现在这就更优雅了。
		hub.commsLock.Lock()
if hub.commsPend > 0 { //正在等待确认，不刷新
			hub.commsLock.Unlock()
			return
		}
	}
	for _, info := range hid.Enumerate(hub.vendorID, 0) {
		for _, id := range hub.productIDs {
			if info.ProductID == id && (info.UsagePage == hub.usageID || info.Interface == hub.endpointID) {
				devices = append(devices, info)
				break
			}
		}
	}
	if runtime.GOOS == "linux" {
//请参阅枚举前的基本原理，了解为什么只在Linux上需要这样做。
		hub.commsLock.Unlock()
	}
//将当前钱包列表转换为新列表
	hub.stateLock.Lock()

	wallets := make([]accounts.Wallet, 0, len(devices))
	events := []accounts.WalletEvent{}

	for _, device := range devices {
		url := accounts.URL{Scheme: hub.scheme, Path: device.Path}

//将钱包放在下一个设备或因某种原因失败的设备前面
		for len(hub.wallets) > 0 {
//如果我们超过当前设备并找到可操作的设备，则中止
			_, failure := hub.wallets[0].Status()
			if hub.wallets[0].URL().Cmp(url) >= 0 || failure == nil {
				break
			}
//丢弃陈旧和故障的设备
			events = append(events, accounts.WalletEvent{Wallet: hub.wallets[0], Kind: accounts.WalletDropped})
			hub.wallets = hub.wallets[1:]
		}
//如果没有更多钱包或设备在下一个之前，请包装新钱包。
		if len(hub.wallets) == 0 || hub.wallets[0].URL().Cmp(url) > 0 {
			logger := log.New("url", url)
			wallet := &wallet{hub: hub, driver: hub.makeDriver(logger), url: &url, info: device, log: logger}

			events = append(events, accounts.WalletEvent{Wallet: wallet, Kind: accounts.WalletArrived})
			wallets = append(wallets, wallet)
			continue
		}
//如果设备与第一个钱包相同，请保留它
		if hub.wallets[0].URL().Cmp(url) == 0 {
			wallets = append(wallets, hub.wallets[0])
			hub.wallets = hub.wallets[1:]
			continue
		}
	}
//扔掉所有剩余的钱包，并设置新的一批
	for _, wallet := range hub.wallets {
		events = append(events, accounts.WalletEvent{Wallet: wallet, Kind: accounts.WalletDropped})
	}
	hub.refreshed = time.Now()
	hub.wallets = wallets
	hub.stateLock.Unlock()

//启动所有钱包事件并返回
	for _, event := range events {
		hub.updateFeed.Send(event)
	}
}

//subscribe实现accounts.backend，创建对的异步订阅
//接收有关添加或删除USB钱包的通知。
func (hub *Hub) Subscribe(sink chan<- accounts.WalletEvent) event.Subscription {
//我们需要mutex来可靠地启动/停止更新循环
	hub.stateLock.Lock()
	defer hub.stateLock.Unlock()

//订阅调用方并跟踪订阅方计数
	sub := hub.updateScope.Track(hub.updateFeed.Subscribe(sink))

//订阅服务器需要一个活动的通知循环，启动它
	if !hub.updating {
		hub.updating = true
		go hub.updater()
	}
	return sub
}

//更新程序负责维护管理的钱包的最新列表
//通过USB集线器启动钱包添加/删除事件。
func (hub *Hub) updater() {
	for {
//TODO:等待USB热插拔事件（尚不支持）或刷新超时
//<Hub变化
		time.Sleep(refreshCycle)

//运行钱包刷新程序
		hub.refreshWallets()

//如果我们所有的订户都离开了，请停止更新程序
		hub.stateLock.Lock()
		if hub.updateScope.Count() == 0 {
			hub.updating = false
			hub.stateLock.Unlock()
			return
		}
		hub.stateLock.Unlock()
	}
}
