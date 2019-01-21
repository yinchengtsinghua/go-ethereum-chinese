
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

package shhclient

import (
	"context"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	whisper "github.com/ethereum/go-ethereum/whisper/whisperv6"
)

//客户端为WhisperV6 RPC API定义类型化包装器。
type Client struct {
	c *rpc.Client
}

//拨号将客户机连接到给定的URL。
func Dial(rawurl string) (*Client, error) {
	c, err := rpc.Dial(rawurl)
	if err != nil {
		return nil, err
	}
	return NewClient(c), nil
}

//newclient创建使用给定RPC客户端的客户端。
func NewClient(c *rpc.Client) *Client {
	return &Client{c}
}

//version返回耳语子协议版本。
func (sc *Client) Version(ctx context.Context) (string, error) {
	var result string
	err := sc.c.CallContext(ctx, &result, "shh_version")
	return result, err
}

//信息返回关于耳语节点的诊断信息。
func (sc *Client) Info(ctx context.Context) (whisper.Info, error) {
	var info whisper.Info
	err := sc.c.CallContext(ctx, &info, "shh_info")
	return info, err
}

//setmaxmessagesize设置此节点允许的最大消息大小。进来的
//较大的外发邮件将被拒绝。低语消息大小
//不能超过基础P2P协议（10 MB）所施加的限制。
func (sc *Client) SetMaxMessageSize(ctx context.Context, size uint32) error {
	var ignored bool
	return sc.c.CallContext(ctx, &ignored, "shh_setMaxMessageSize", size)
}

//setminimumPow（实验）设置此节点所需的最小功率。
//该实验函数被引入到未来的动态调节中。
//功率要求。如果节点被消息淹没，它应该引发
//POW要求并通知同行。新值应设置为相对于
//旧值（例如double）。旧值可以通过shh_info调用获得。
func (sc *Client) SetMinimumPoW(ctx context.Context, pow float64) error {
	var ignored bool
	return sc.c.CallContext(ctx, &ignored, "shh_setMinPoW", pow)
}

//marktrustedpeer标记特定的受信任的对等机，这将允许它发送历史（过期）消息。
//注意：此功能不添加新节点，节点需要作为对等节点存在。
func (sc *Client) MarkTrustedPeer(ctx context.Context, enode string) error {
	var ignored bool
	return sc.c.CallContext(ctx, &ignored, "shh_markTrustedPeer", enode)
}

//new key pair为消息解密和加密生成一个新的公钥和私钥对。
//它返回一个可用于引用键的标识符。
func (sc *Client) NewKeyPair(ctx context.Context) (string, error) {
	var id string
	return id, sc.c.CallContext(ctx, &id, "shh_newKeyPair")
}

//addprivatekey存储了密钥对，并返回其ID。
func (sc *Client) AddPrivateKey(ctx context.Context, key []byte) (string, error) {
	var id string
	return id, sc.c.CallContext(ctx, &id, "shh_addPrivateKey", hexutil.Bytes(key))
}

//删除密钥对删除指定密钥。
func (sc *Client) DeleteKeyPair(ctx context.Context, id string) (string, error) {
	var ignored bool
	return id, sc.c.CallContext(ctx, &ignored, "shh_deleteKeyPair", id)
}

//hasKeyPair返回节点是否具有私钥或
//与给定ID匹配的密钥对。
func (sc *Client) HasKeyPair(ctx context.Context, id string) (bool, error) {
	var has bool
	return has, sc.c.CallContext(ctx, &has, "shh_hasKeyPair", id)
}

//public key返回密钥ID的公钥。
func (sc *Client) PublicKey(ctx context.Context, id string) ([]byte, error) {
	var key hexutil.Bytes
	return []byte(key), sc.c.CallContext(ctx, &key, "shh_getPublicKey", id)
}

//private key返回密钥ID的私钥。
func (sc *Client) PrivateKey(ctx context.Context, id string) ([]byte, error) {
	var key hexutil.Bytes
	return []byte(key), sc.c.CallContext(ctx, &key, "shh_getPrivateKey", id)
}

//NewSymmetricKey生成随机对称密钥并返回其标识符。
//可用于加密和解密双方都知道密钥的消息。
func (sc *Client) NewSymmetricKey(ctx context.Context) (string, error) {
	var id string
	return id, sc.c.CallContext(ctx, &id, "shh_newSymKey")
}

//addSymmetricKey存储密钥，并返回其标识符。
func (sc *Client) AddSymmetricKey(ctx context.Context, key []byte) (string, error) {
	var id string
	return id, sc.c.CallContext(ctx, &id, "shh_addSymKey", hexutil.Bytes(key))
}

//generatesymmetrickeyfrompassword根据密码生成密钥，存储并返回其标识符。
func (sc *Client) GenerateSymmetricKeyFromPassword(ctx context.Context, passwd string) (string, error) {
	var id string
	return id, sc.c.CallContext(ctx, &id, "shh_generateSymKeyFromPassword", passwd)
}

//hassymmetrickey返回与给定ID关联的密钥是否存储在节点中的指示。
func (sc *Client) HasSymmetricKey(ctx context.Context, id string) (bool, error) {
	var found bool
	return found, sc.c.CallContext(ctx, &found, "shh_hasSymKey", id)
}

//GetSymmetricKey返回与给定标识符关联的对称密钥。
func (sc *Client) GetSymmetricKey(ctx context.Context, id string) ([]byte, error) {
	var key hexutil.Bytes
	return []byte(key), sc.c.CallContext(ctx, &key, "shh_getSymKey", id)
}

//DeleteSymmetricKey删除与给定标识符关联的对称密钥。
func (sc *Client) DeleteSymmetricKey(ctx context.Context, id string) error {
	var ignored bool
	return sc.c.CallContext(ctx, &ignored, "shh_deleteSymKey", id)
}

//在网络上发布消息。
func (sc *Client) Post(ctx context.Context, message whisper.NewMessage) (string, error) {
	var hash string
	return hash, sc.c.CallContext(ctx, &hash, "shh_post", message)
}

//订阅消息订阅与给定条件匹配的消息。这种方法
//仅在双向连接（如WebSockets和IPC）上受支持。
//NewMessageFilter使用轮询，并且通过HTTP支持。
func (sc *Client) SubscribeMessages(ctx context.Context, criteria whisper.Criteria, ch chan<- *whisper.Message) (ethereum.Subscription, error) {
	return sc.c.ShhSubscribe(ctx, ch, "messages", criteria)
}

//newMessageFilter在节点内创建一个筛选器。此筛选器可用于轮询
//对于满足给定条件的新消息（请参阅filtermessages）。过滤器罐
//在whisper.filterTimeout中对其进行轮询时超时。
func (sc *Client) NewMessageFilter(ctx context.Context, criteria whisper.Criteria) (string, error) {
	var id string
	return id, sc.c.CallContext(ctx, &id, "shh_newMessageFilter", criteria)
}

//DeleteMessageFilter删除与给定ID关联的筛选器。
func (sc *Client) DeleteMessageFilter(ctx context.Context, id string) error {
	var ignored bool
	return sc.c.CallContext(ctx, &ignored, "shh_deleteMessageFilter", id)
}

//filtermessages检索上次调用之间接收的所有消息
//此函数与创建筛选器时给定的条件匹配。
func (sc *Client) FilterMessages(ctx context.Context, id string) ([]*whisper.Message, error) {
	var messages []*whisper.Message
	return messages, sc.c.CallContext(ctx, &messages, "shh_getFilterMessages", id)
}
