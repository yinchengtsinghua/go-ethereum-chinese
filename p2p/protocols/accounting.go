
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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

package protocols

import (
	"time"

	"github.com/ethereum/go-ethereum/metrics"
)

//定义一些指标
var (
//所有指标都是累积的

//单位贷记总额
	mBalanceCredit metrics.Counter
//借记单位总额
	mBalanceDebit metrics.Counter
//贷记字节总数
	mBytesCredit metrics.Counter
//借记字节总数
	mBytesDebit metrics.Counter
//贷记邮件总数
	mMsgCredit metrics.Counter
//借记邮件总数
	mMsgDebit metrics.Counter
//本地节点必须删除远程对等节点的次数
	mPeerDrops metrics.Counter
//本地节点透支和丢弃的次数
	mSelfDrops metrics.Counter

	MetricsRegistry metrics.Registry
)

//价格定义如何将价格传递给会计实例
type Prices interface {
//返回消息的价格
	Price(interface{}) *Price
}

type Payer bool

const (
	Sender   = Payer(true)
	Receiver = Payer(false)
)

//价格表示消息的成本
type Price struct {
	Value   uint64
PerByte bool //如果价格为每字节或单位，则为真
	Payer   Payer
}

//因为给了信息的价格
//协议以绝对值提供消息价格
//然后，此方法返回正确的有符号金额，
//根据“付款人”论点确定的付款人：
//“send”将传递“sender”付款人，“receive”将传递“receiver”参数。
//因此：如果发送人和发送人支付，金额为正，否则为负。
//如果收货方付款，金额为正，否则为负。
func (p *Price) For(payer Payer, size uint32) int64 {
	price := p.Value
	if p.PerByte {
		price *= uint64(size)
	}
	if p.Payer == payer {
		return 0 - int64(price)
	}
	return int64(price)
}

//余额是实际的会计实例
//余额定义了会计所需的操作
//实现在内部维护每个对等点的平衡
type Balance interface {
//使用远程节点“peer”将金额添加到本地余额中；
//正数=贷方本地节点
//负金额=借方本地节点
	Add(amount int64, peer *Peer) error
}

//会计实现钩子接口
//它通过余额接口与余额接口连接，
//通过价格接口与协议及其价格进行接口时
type Accounting struct {
Balance //会计逻辑接口
Prices  //价格逻辑接口
}

func NewAccounting(balance Balance, po Prices) *Accounting {
	ah := &Accounting{
		Prices:  po,
		Balance: balance,
	}
	return ah
}

//SetupAccountingMetrics为P2P会计指标创建单独的注册表；
//这个注册表应该独立于任何其他指标，因为它在不同的端点上持续存在。
//它还实例化给定的度量并启动持续执行例程，该例程
//在经过的时间间隔内，将度量值写入级别数据库
func SetupAccountingMetrics(reportInterval time.Duration, path string) *AccountingMetrics {
//创建空注册表
	MetricsRegistry = metrics.NewRegistry()
//实例化度量
	mBalanceCredit = metrics.NewRegisteredCounterForced("account.balance.credit", MetricsRegistry)
	mBalanceDebit = metrics.NewRegisteredCounterForced("account.balance.debit", MetricsRegistry)
	mBytesCredit = metrics.NewRegisteredCounterForced("account.bytes.credit", MetricsRegistry)
	mBytesDebit = metrics.NewRegisteredCounterForced("account.bytes.debit", MetricsRegistry)
	mMsgCredit = metrics.NewRegisteredCounterForced("account.msg.credit", MetricsRegistry)
	mMsgDebit = metrics.NewRegisteredCounterForced("account.msg.debit", MetricsRegistry)
	mPeerDrops = metrics.NewRegisteredCounterForced("account.peerdrops", MetricsRegistry)
	mSelfDrops = metrics.NewRegisteredCounterForced("account.selfdrops", MetricsRegistry)
//创建数据库并开始持久化
	return NewAccountingMetrics(MetricsRegistry, reportInterval, path)
}

//发送需要一个对等点、一个大小和一个消息以及
//-使用价格接口计算本地节点向对等端发送大小消息的成本
//-使用余额界面的贷记/借记本地节点
func (ah *Accounting) Send(peer *Peer, size uint32, msg interface{}) error {
//获取消息的价格（通过协议规范）
	price := ah.Price(msg)
//此邮件不需要记帐
	if price == nil {
		return nil
	}
//评估发送消息的价格
	costToLocalNode := price.For(Sender, size)
//做会计工作
	err := ah.Add(costToLocalNode, peer)
//记录度量：只增加面向用户的度量的计数器
	ah.doMetrics(costToLocalNode, size, err)
	return err
}

//接收需要一个对等点、一个大小和一个消息以及
//-使用价格接口计算从对等端接收大小消息的本地节点的成本
//-使用余额界面的贷记/借记本地节点
func (ah *Accounting) Receive(peer *Peer, size uint32, msg interface{}) error {
//获取消息的价格（通过协议规范）
	price := ah.Price(msg)
//此邮件不需要记帐
	if price == nil {
		return nil
	}
//评估接收消息的价格
	costToLocalNode := price.For(Receiver, size)
//做会计工作
	err := ah.Add(costToLocalNode, peer)
//记录度量：只增加面向用户的度量的计数器
	ah.doMetrics(costToLocalNode, size, err)
	return err
}

//记录一些指标
//这不是错误处理。'err'由'send'和'receive'返回
//如果违反了限制（透支），则“err”将不为零，在这种情况下，对等方已被取消。
//如果违反了限制，“err”不等于零：
//*如果价格为正数，则本地节点已记入贷方；因此“err”隐式地表示远程节点已被删除。
//*如果价格为负，则本地节点已被借记，因此“err”隐式地表示本地节点“透支”
func (ah *Accounting) doMetrics(price int64, size uint32, err error) {
	if price > 0 {
		mBalanceCredit.Inc(price)
		mBytesCredit.Inc(int64(size))
		mMsgCredit.Inc(1)
		if err != nil {
//增加由于“透支”而丢弃远程节点的次数
			mPeerDrops.Inc(1)
		}
	} else {
		mBalanceDebit.Inc(price)
		mBytesDebit.Inc(int64(size))
		mMsgDebit.Inc(1)
		if err != nil {
//增加本地节点对其他节点进行“透支”的次数
			mSelfDrops.Inc(1)
		}
	}
}
