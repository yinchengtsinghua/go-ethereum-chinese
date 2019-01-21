
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

/*
软件包Whisperv5实现了Whisper协议（版本5）。

Whisper结合了DHTS和数据报消息系统（如UDP）的各个方面。
因此，可以将其与两者进行比较，而不是与
物质/能量二元性（为明目张胆地滥用
基本而美丽的自然法则）。

Whisper是一个纯粹的基于身份的消息传递系统。低语提供了一个低层次
（非特定于应用程序）但不基于
或者受到低级硬件属性和特性的影响，
尤其是奇点的概念。
**/

package whisperv5

import (
	"fmt"
	"time"
)

const (
	EnvelopeVersion    = uint64(0)
	ProtocolVersion    = uint64(5)
	ProtocolVersionStr = "5.0"
	ProtocolName       = "shh"

statusCode           = 0 //由耳语协议使用
messagesCode         = 1 //正常低语信息
p2pCode              = 2 //对等消息（由对等方使用，但不再转发）
p2pRequestCode       = 3 //点对点消息，由DAPP协议使用
	NumberOfMessageCodes = 64

	paddingMask   = byte(3)
	signatureFlag = byte(4)

	TopicLength     = 4
	signatureLength = 65
	aesKeyLength    = 32
	AESNonceLength  = 12
	keyIdSize       = 32

MaxMessageSize        = uint32(10 * 1024 * 1024) //邮件的最大可接受大小。
	DefaultMaxMessageSize = uint32(1024 * 1024)
	DefaultMinimumPoW     = 0.2

padSizeLimit      = 256 //只是一个任意数字，可以在不破坏协议的情况下进行更改（不得超过2^24）
	messageQueueLimit = 1024

	expirationCycle   = time.Second
	transmissionCycle = 300 * time.Millisecond

DefaultTTL     = 50 //秒
SynchAllowance = 10 //秒
)

type unknownVersionError uint64

func (e unknownVersionError) Error() string {
	return fmt.Sprintf("invalid envelope version %d", uint64(e))
}

//mail server表示一个邮件服务器，能够
//存档旧邮件以供后续传递
//对同龄人。任何实施都必须确保
//函数是线程安全的。而且，他们必须尽快返回。
//delivermail应使用directmessagescode进行传递，
//以绕过到期检查。
type MailServer interface {
	Archive(env *Envelope)
	DeliverMail(whisperPeer *Peer, request *Envelope)
}
