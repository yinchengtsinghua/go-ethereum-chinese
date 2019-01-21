
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

package swap

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/swarm/log"
)

//交换Swarm会计协议
//SWIFT自动支付
//点对点小额支付系统

//配置文件-公共交换配置文件
//交换的公共参数，握手中传递的可序列化配置结构
type Profile struct {
BuyAt  *big.Int //接受的块最大价格
SellAt *big.Int //大宗商品报价
PayAt  uint     //触发付款请求的阈值
DropAt uint     //触发断开连接的阈值
}

//策略封装了与
//自动存款和自动兑现
type Strategy struct {
AutoCashInterval     time.Duration //自动清除的默认间隔
AutoCashThreshold    *big.Int      //触发自动清除的阈值（wei）
AutoDepositInterval  time.Duration //自动清除的默认间隔
AutoDepositThreshold *big.Int      //触发自动报告的阈值（wei）
AutoDepositBuffer    *big.Int      //剩余用于叉保护等的缓冲器（WEI）
}

//params使用与
//自动存款和自动兑现
type Params struct {
	*Profile
	*Strategy
}

//承诺-第三方可证明的付款承诺
//由预付款签发
//可序列化以使用协议发送
type Promise interface{}

//用于测试或外部替代支付的对等协议的协议接口
type Protocol interface {
Pay(int, Promise) //单位，付款证明
	Drop()
	String() string
}

//自动存款（延迟）出站支付系统的出站支付接口
type OutPayment interface {
	Issue(amount *big.Int) (promise Promise, err error)
	AutoDeposit(interval time.Duration, threshold, buffer *big.Int)
	Stop()
}

//自动刷卡（延迟）入站支付系统的支付接口
type InPayment interface {
	Receive(promise Promise) (*big.Int, error)
	AutoCash(cashInterval time.Duration, maxUncashed *big.Int)
	Stop()
}

//swap是swarm会计协议实例
//*成对会计和付款
type Swap struct {
lock    sync.Mutex //用于平衡访问的互斥
balance int        //块/检索请求的单位
local   *Params    //本地对等交换参数
remote  *Profile   //远程对等交换配置文件
proto   Protocol   //对等通信协议
	Payment
}

//付款处理程序
type Payment struct {
Out         OutPayment //传出付款处理程序
In          InPayment  //入站付款经办人
	Buys, Sells bool
}

//新建-交换构造函数
func New(local *Params, pm Payment, proto Protocol) (swap *Swap, err error) {

	swap = &Swap{
		local:   local,
		Payment: pm,
		proto:   proto,
	}

	swap.SetParams(local)

	return
}

//setremote-设置远程交换配置文件的入口点（例如来自握手或其他消息）
func (swap *Swap) SetRemote(remote *Profile) {
	defer swap.lock.Unlock()
	swap.lock.Lock()

	swap.remote = remote
	if swap.Sells && (remote.BuyAt.Sign() <= 0 || swap.local.SellAt.Sign() <= 0 || remote.BuyAt.Cmp(swap.local.SellAt) < 0) {
		swap.Out.Stop()
		swap.Sells = false
	}
	if swap.Buys && (remote.SellAt.Sign() <= 0 || swap.local.BuyAt.Sign() <= 0 || swap.local.BuyAt.Cmp(swap.remote.SellAt) < 0) {
		swap.In.Stop()
		swap.Buys = false
	}

	log.Debug(fmt.Sprintf("<%v> remote profile set: pay at: %v, drop at: %v, buy at: %v, sell at: %v", swap.proto, remote.PayAt, remote.DropAt, remote.BuyAt, remote.SellAt))

}

//setparams-动态设置策略
func (swap *Swap) SetParams(local *Params) {
	defer swap.lock.Unlock()
	swap.lock.Lock()
	swap.local = local
	swap.setParams(local)
}

//setparams-调用方持有锁
func (swap *Swap) setParams(local *Params) {

	if swap.Sells {
		swap.In.AutoCash(local.AutoCashInterval, local.AutoCashThreshold)
		log.Info(fmt.Sprintf("<%v> set autocash to every %v, max uncashed limit: %v", swap.proto, local.AutoCashInterval, local.AutoCashThreshold))
	} else {
		log.Info(fmt.Sprintf("<%v> autocash off (not selling)", swap.proto))
	}
	if swap.Buys {
		swap.Out.AutoDeposit(local.AutoDepositInterval, local.AutoDepositThreshold, local.AutoDepositBuffer)
		log.Info(fmt.Sprintf("<%v> set autodeposit to every %v, pay at: %v, buffer: %v", swap.proto, local.AutoDepositInterval, local.AutoDepositThreshold, local.AutoDepositBuffer))
	} else {
		log.Info(fmt.Sprintf("<%v> autodeposit off (not buying)", swap.proto))
	}
}

//加法（n）
//当承诺/提供n个服务单元时调用n>0
//n<0使用/请求时调用n个服务单元
func (swap *Swap) Add(n int) error {
	defer swap.lock.Unlock()
	swap.lock.Lock()
	swap.balance += n
	if !swap.Sells && swap.balance > 0 {
		log.Trace(fmt.Sprintf("<%v> remote peer cannot have debt (balance: %v)", swap.proto, swap.balance))
		swap.proto.Drop()
		return fmt.Errorf("[SWAP] <%v> remote peer cannot have debt (balance: %v)", swap.proto, swap.balance)
	}
	if !swap.Buys && swap.balance < 0 {
		log.Trace(fmt.Sprintf("<%v> we cannot have debt (balance: %v)", swap.proto, swap.balance))
		return fmt.Errorf("[SWAP] <%v> we cannot have debt (balance: %v)", swap.proto, swap.balance)
	}
	if swap.balance >= int(swap.local.DropAt) {
		log.Trace(fmt.Sprintf("<%v> remote peer has too much debt (balance: %v, disconnect threshold: %v)", swap.proto, swap.balance, swap.local.DropAt))
		swap.proto.Drop()
		return fmt.Errorf("[SWAP] <%v> remote peer has too much debt (balance: %v, disconnect threshold: %v)", swap.proto, swap.balance, swap.local.DropAt)
	} else if swap.balance <= -int(swap.remote.PayAt) {
		swap.send()
	}
	return nil
}

//余额存取器
func (swap *Swap) Balance() int {
	defer swap.lock.Unlock()
	swap.lock.Lock()
	return swap.balance
}

//付款到期时调用发送（单位）
//在破产的情况下，不签发和发送任何承诺，以防欺诈。
//无返回值：无错误=付款是机会主义的=挂起直到丢弃
func (swap *Swap) send() {
	if swap.local.BuyAt != nil && swap.balance < 0 {
		amount := big.NewInt(int64(-swap.balance))
		amount.Mul(amount, swap.remote.SellAt)
		promise, err := swap.Out.Issue(amount)
		if err != nil {
			log.Warn(fmt.Sprintf("<%v> cannot issue cheque (amount: %v, channel: %v): %v", swap.proto, amount, swap.Out, err))
		} else {
			log.Warn(fmt.Sprintf("<%v> cheque issued (amount: %v, channel: %v)", swap.proto, amount, swap.Out))
			swap.proto.Pay(-swap.balance, promise)
			swap.balance = 0
		}
	}
}

//当收到付款消息时，协议调用Receive（Units，Promise）
//如果承诺无效，则返回错误。
func (swap *Swap) Receive(units int, promise Promise) error {
	if units <= 0 {
		return fmt.Errorf("invalid units: %v <= 0", units)
	}

	price := new(big.Int).SetInt64(int64(units))
	price.Mul(price, swap.local.SellAt)

	amount, err := swap.In.Receive(promise)

	if err != nil {
		err = fmt.Errorf("invalid promise: %v", err)
	} else if price.Cmp(amount) != 0 {
//核实金额=单位*单位售价
		return fmt.Errorf("invalid amount: %v = %v * %v (units sent in msg * agreed sale unit price) != %v (signed in cheque)", price, units, swap.local.SellAt, amount)
	}
	if err != nil {
		log.Trace(fmt.Sprintf("<%v> invalid promise (amount: %v, channel: %v): %v", swap.proto, amount, swap.In, err))
		return err
	}

//带单位的信用远程对等
	swap.Add(-units)
	log.Trace(fmt.Sprintf("<%v> received promise (amount: %v, channel: %v): %v", swap.proto, amount, swap.In, promise))

	return nil
}

//停止会导致autocash循环终止。
//在协议句柄循环终止后调用。
func (swap *Swap) Stop() {
	defer swap.lock.Unlock()
	swap.lock.Lock()
	if swap.Buys {
		swap.Out.Stop()
	}
	if swap.Sells {
		swap.In.Stop()
	}
}
