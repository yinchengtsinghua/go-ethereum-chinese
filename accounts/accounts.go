
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

//软件包帐户实现了高级以太坊帐户管理。
package accounts

import (
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

//帐户表示位于定义的特定位置的以太坊帐户
//通过可选的URL字段。
type Account struct {
Address common.Address `json:"address"` //从密钥派生的以太坊帐户地址
URL     URL            `json:"url"`     //后端中的可选资源定位器
}

//Wallet表示可能包含一个或多个软件或硬件钱包
//账户（源自同一种子）。
type Wallet interface {
//URL检索可访问此钱包的规范路径。它是
//用户按上层定义多个钱包的排序顺序
//后端。
	URL() URL

//状态返回文本状态以帮助用户处于
//钱包。它还返回一个错误，指示钱包可能发生的任何故障。
//遇到。
	Status() (string, error)

//open初始化对钱包实例的访问。它不是用来解锁或
//解密帐户密钥，而不是简单地建立到硬件的连接
//钱包和/或获取衍生种子。
//
//passphrase参数可能被
//特定钱包实例。没有无密码打开方法的原因
//是为了争取一个统一的钱包处理，忘却了不同
//后端提供程序。
//
//请注意，如果您打开一个钱包，您必须关闭它以释放任何分配的
//资源（使用硬件钱包时尤其重要）。
	Open(passphrase string) error

//关闭释放打开钱包实例持有的任何资源。
	Close() error

//帐户检索钱包当前识别的签名帐户列表
//的。对于等级决定论钱包，列表不会是详尽的，
//而是只包含在帐户派生期间显式固定的帐户。
	Accounts() []Account

//包含返回帐户是否属于此特定钱包的一部分。
	Contains(account Account) bool

//派生尝试在处显式派生层次确定性帐户
//指定的派生路径。如果请求，将添加派生帐户
//到钱包的跟踪帐户列表。
	Derive(path DerivationPath, pin bool) (Account, error)

//SelfDerive设置钱包尝试的基本帐户派生路径
//发现非零帐户并自动将其添加到跟踪的列表
//账户。
//
//注意，自派生将增加指定路径的最后一个组件
//与减少到子路径以允许发现开始的帐户相反
//来自非零组件。
//
//您可以通过使用nil调用selfderive来禁用自动帐户发现
//链状态读取器。
	SelfDerive(base DerivationPath, chain ethereum.ChainStateReader)

//sign hash请求钱包对给定的hash进行签名。
//
//它只通过包含在中的地址查找指定的帐户，
//或者可以借助嵌入的URL字段中的任何位置元数据。
//
//如果钱包需要额外的认证来签署请求（例如
//用于解密帐户的密码，或用于验证事务的PIN代码）。
//将返回一个authneedederror实例，其中包含用户的信息
//关于需要哪些字段或操作。用户可以通过提供
//通过带密码的signhash或其他方式（例如解锁）获得所需的详细信息
//密钥库中的帐户）。
	SignHash(account Account, hash []byte) ([]byte, error)

//signtx请求钱包签署给定的交易。
//
//它只通过包含在中的地址查找指定的帐户，
//或者可以借助嵌入的URL字段中的任何位置元数据。
//
//如果钱包需要额外的认证来签署请求（例如
//用于解密帐户的密码，或用于验证事务的PIN代码）。
//将返回一个authneedederror实例，其中包含用户的信息
//关于需要哪些字段或操作。用户可以通过提供
//通过带密码的SIGNTX或其他方式（例如解锁）获得所需的详细信息
//密钥库中的帐户）。
	SignTx(account Account, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error)

//signhashwithpassphrase请求钱包使用
//提供作为额外身份验证信息的密码。
//
//它只通过包含在中的地址查找指定的帐户，
//或者可以借助嵌入的URL字段中的任何位置元数据。
	SignHashWithPassphrase(account Account, passphrase string, hash []byte) ([]byte, error)

//signtxwithpassphrase请求钱包签署给定的交易，使用
//提供作为额外身份验证信息的密码。
//
//它只通过包含在中的地址查找指定的帐户，
//或者可以借助嵌入的URL字段中的任何位置元数据。
	SignTxWithPassphrase(account Account, passphrase string, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error)
}

//后端是一个“钱包提供商”，可能包含他们可以使用的一批账户。
//根据要求签署交易。
type Backend interface {
//Wallets检索后端当前已知的钱包列表。
//
//默认情况下不会打开返回的钱包。对于软件高清钱包
//意味着没有基本种子被解密，对于硬件钱包，没有实际的
//已建立连接。
//
//生成的钱包列表将根据其内部的字母顺序进行排序。
//后端分配的URL。因为钱包（尤其是硬件）可能会出现
//去吧，同一个钱包可能会出现在列表中的不同位置
//后续检索。
	Wallets() []Wallet

//订阅创建异步订阅以在
//后端检测钱包的到达或离开。
	Subscribe(sink chan<- WalletEvent) event.Subscription
}

//WalletEventType表示可以由
//钱包订阅子系统。
type WalletEventType int

const (
//当通过USB或通过
//密钥库中的文件系统事件。
	WalletArrived WalletEventType = iota

//当钱包成功打开时，Walletopened会被触发。
//启动任何后台进程，如自动密钥派生。
	WalletOpened

//壁纸
	WalletDropped
)

//WalletEvent是当钱包到达或
//检测到离场。
type WalletEvent struct {
Wallet Wallet          //钱包实例到达或离开
Kind   WalletEventType //系统中发生的事件类型
}
