
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

//包含用于Whisper客户端的包装。

package geth

import (
	"github.com/ethereum/go-ethereum/whisper/shhclient"
	whisper "github.com/ethereum/go-ethereum/whisper/whisperv6"
)

//WhisperClient提供对以太坊API的访问。
type WhisperClient struct {
	client *shhclient.Client
}

//NewWhisperClient将客户机连接到给定的URL。
func NewWhisperClient(rawurl string) (client *WhisperClient, _ error) {
	rawClient, err := shhclient.Dial(rawurl)
	return &WhisperClient{rawClient}, err
}

//GetVersion返回Whisper子协议版本。
func (wc *WhisperClient) GetVersion(ctx *Context) (version string, _ error) {
	return wc.client.Version(ctx.context)
}

//信息返回关于耳语节点的诊断信息。
func (wc *WhisperClient) GetInfo(ctx *Context) (info *Info, _ error) {
	rawInfo, err := wc.client.Info(ctx.context)
	return &Info{&rawInfo}, err
}

//setmaxmessagesize设置此节点允许的最大消息大小。进来的
//较大的外发邮件将被拒绝。低语消息大小
//不能超过基础P2P协议（10 MB）所施加的限制。
func (wc *WhisperClient) SetMaxMessageSize(ctx *Context, size int32) error {
	return wc.client.SetMaxMessageSize(ctx.context, uint32(size))
}

//setminimumPow（实验）设置此节点所需的最小功率。
//该实验函数被引入到未来的动态调节中。
//功率要求。如果节点被消息淹没，它应该引发
//POW要求并通知同行。新值应设置为相对于
//旧值（例如double）。旧值可以通过shh_info调用获得。
func (wc *WhisperClient) SetMinimumPoW(ctx *Context, pow float64) error {
	return wc.client.SetMinimumPoW(ctx.context, pow)
}

//将特定的对等机标记为可信，这将允许它发送历史（过期）消息。
//注意：此功能不添加新节点，节点需要作为对等节点存在。
func (wc *WhisperClient) MarkTrustedPeer(ctx *Context, enode string) error {
	return wc.client.MarkTrustedPeer(ctx.context, enode)
}

//new key pair为消息解密和加密生成一个新的公钥和私钥对。
//它返回一个可用于引用键的标识符。
func (wc *WhisperClient) NewKeyPair(ctx *Context) (string, error) {
	return wc.client.NewKeyPair(ctx.context)
}

//addprivatekey存储了密钥对，并返回其ID。
func (wc *WhisperClient) AddPrivateKey(ctx *Context, key []byte) (string, error) {
	return wc.client.AddPrivateKey(ctx.context, key)
}

//删除密钥对删除指定密钥。
func (wc *WhisperClient) DeleteKeyPair(ctx *Context, id string) (string, error) {
	return wc.client.DeleteKeyPair(ctx.context, id)
}

//hasKeyPair返回节点是否具有私钥或
//与给定ID匹配的密钥对。
func (wc *WhisperClient) HasKeyPair(ctx *Context, id string) (bool, error) {
	return wc.client.HasKeyPair(ctx.context, id)
}

//GetPublicKey返回键ID的公钥。
func (wc *WhisperClient) GetPublicKey(ctx *Context, id string) ([]byte, error) {
	return wc.client.PublicKey(ctx.context, id)
}

//getprivatekey返回密钥ID的私钥。
func (wc *WhisperClient) GetPrivateKey(ctx *Context, id string) ([]byte, error) {
	return wc.client.PrivateKey(ctx.context, id)
}

//NewSymmetricKey生成随机对称密钥并返回其标识符。
//可用于加密和解密双方都知道密钥的消息。
func (wc *WhisperClient) NewSymmetricKey(ctx *Context) (string, error) {
	return wc.client.NewSymmetricKey(ctx.context)
}

//addSymmetricKey存储密钥，并返回其标识符。
func (wc *WhisperClient) AddSymmetricKey(ctx *Context, key []byte) (string, error) {
	return wc.client.AddSymmetricKey(ctx.context, key)
}

//generatesymmetrickeyfrompassword根据密码生成密钥，存储并返回其标识符。
func (wc *WhisperClient) GenerateSymmetricKeyFromPassword(ctx *Context, passwd string) (string, error) {
	return wc.client.GenerateSymmetricKeyFromPassword(ctx.context, passwd)
}

//hassymmetrickey返回与给定ID关联的密钥是否存储在节点中的指示。
func (wc *WhisperClient) HasSymmetricKey(ctx *Context, id string) (bool, error) {
	return wc.client.HasSymmetricKey(ctx.context, id)
}

//GetSymmetricKey返回与给定标识符关联的对称密钥。
func (wc *WhisperClient) GetSymmetricKey(ctx *Context, id string) ([]byte, error) {
	return wc.client.GetSymmetricKey(ctx.context, id)
}

//DeleteSymmetricKey删除与给定标识符关联的对称密钥。
func (wc *WhisperClient) DeleteSymmetricKey(ctx *Context, id string) error {
	return wc.client.DeleteSymmetricKey(ctx.context, id)
}

//在网络上发布消息。
func (wc *WhisperClient) Post(ctx *Context, message *NewMessage) (string, error) {
	return wc.client.Post(ctx.context, *message.newMessage)
}

//NewHeadHandler是一个客户端订阅回调，用于在事件和
//订阅失败。
type NewMessageHandler interface {
	OnNewMessage(message *Message)
	OnError(failure string)
}

//订阅消息订阅与给定条件匹配的消息。这种方法
//仅在双向连接（如WebSockets和IPC）上受支持。
//NewMessageFilter使用轮询，并且通过HTTP支持。
func (wc *WhisperClient) SubscribeMessages(ctx *Context, criteria *Criteria, handler NewMessageHandler, buffer int) (*Subscription, error) {
//在内部订阅事件
	ch := make(chan *whisper.Message, buffer)
	rawSub, err := wc.client.SubscribeMessages(ctx.context, *criteria.criteria, ch)
	if err != nil {
		return nil, err
	}
//启动一个调度器以反馈回拨
	go func() {
		for {
			select {
			case message := <-ch:
				handler.OnNewMessage(&Message{message})

			case err := <-rawSub.Err():
				if err != nil {
					handler.OnError(err.Error())
				}
				return
			}
		}
	}()
	return &Subscription{rawSub}, nil
}

//newMessageFilter在节点内创建一个筛选器。此筛选器可用于轮询
//对于满足给定条件的新消息（请参阅filtermessages）。过滤器罐
//在whisper.filterTimeout中对其进行轮询时超时。
func (wc *WhisperClient) NewMessageFilter(ctx *Context, criteria *Criteria) (string, error) {
	return wc.client.NewMessageFilter(ctx.context, *criteria.criteria)
}

//DeleteMessageFilter删除与给定ID关联的筛选器。
func (wc *WhisperClient) DeleteMessageFilter(ctx *Context, id string) error {
	return wc.client.DeleteMessageFilter(ctx.context, id)
}

//GetFilterMessages检索上次调用之间接收的所有消息
//此函数与创建筛选器时给定的条件匹配。
func (wc *WhisperClient) GetFilterMessages(ctx *Context, id string) (*Messages, error) {
	rawFilterMessages, err := wc.client.FilterMessages(ctx.context, id)
	if err != nil {
		return nil, err
	}
	res := make([]*whisper.Message, len(rawFilterMessages))
	copy(res, rawFilterMessages)
	return &Messages{res}, nil
}
