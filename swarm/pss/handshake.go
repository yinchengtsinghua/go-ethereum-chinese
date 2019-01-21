
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

//+建设！诺普桑德摇晃

package pss

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/log"
)

const (
	IsActiveHandshake = true
)

var (
	ctrlSingleton *HandshakeController
)

const (
defaultSymKeyRequestTimeout = 1000 * 8  //接收对握手符号请求的响应的最大等待毫秒数
defaultSymKeyExpiryTimeout  = 1000 * 10 //ms等待，然后允许垃圾收集过期的symkey
defaultSymKeySendLimit      = 256       //symkey的有效消息量
defaultSymKeyCapacity       = 4         //同时存储/发送的最大符号键数
)

//对称密钥交换消息负载
type handshakeMsg struct {
	From    []byte
	Limit   uint16
	Keys    [][]byte
	Request uint8
	Topic   Topic
}

//单个对称密钥的内部表示法
type handshakeKey struct {
	symKeyID  *string
	pubKeyID  *string
	limit     uint16
	count     uint16
	expiredAt time.Time
}

//所有输入和输出密钥的容器
//对于一个特定的对等（公钥）和主题
type handshake struct {
	outKeys []handshakeKey
	inKeys  []handshakeKey
}

//握手控制器的初始化参数
//
//symkeyrequestexpiry:等待握手回复超时
//（默认8000 ms）
//
//symkeysendlimit:对称密钥问题的消息量
//此节点的有效期为（默认256）
//
//symkeycapacity:对称密钥的理想（和最大）数量
//每个对等端的每个方向保持（默认为4）
type HandshakeParams struct {
	SymKeyRequestTimeout time.Duration
	SymKeyExpiryTimeout  time.Duration
	SymKeySendLimit      uint16
	SymKeyCapacity       uint8
}

//握手控制器初始化的正常默认值
func NewHandshakeParams() *HandshakeParams {
	return &HandshakeParams{
		SymKeyRequestTimeout: defaultSymKeyRequestTimeout * time.Millisecond,
		SymKeyExpiryTimeout:  defaultSymKeyExpiryTimeout * time.Millisecond,
		SymKeySendLimit:      defaultSymKeySendLimit,
		SymKeyCapacity:       defaultSymKeyCapacity,
	}
}

//启用半自动diffie-hellman的singleton对象
//交换短暂对称密钥
type HandshakeController struct {
	pss                  *Pss
keyC                 map[string]chan []string //添加握手成功时要报告的频道
	lock                 sync.Mutex
	symKeyRequestTimeout time.Duration
	symKeyExpiryTimeout  time.Duration
	symKeySendLimit      uint16
	symKeyCapacity       uint8
	symKeyIndex          map[string]*handshakeKey
	handshakes           map[string]map[Topic]*handshake
	deregisterFuncs      map[Topic]func()
}

//将握手控制器连接到PSS节点
//
//必须在启动PSS节点服务之前调用
func SetHandshakeController(pss *Pss, params *HandshakeParams) error {
	ctrl := &HandshakeController{
		pss:                  pss,
		keyC:                 make(map[string]chan []string),
		symKeyRequestTimeout: params.SymKeyRequestTimeout,
		symKeyExpiryTimeout:  params.SymKeyExpiryTimeout,
		symKeySendLimit:      params.SymKeySendLimit,
		symKeyCapacity:       params.SymKeyCapacity,
		symKeyIndex:          make(map[string]*handshakeKey),
		handshakes:           make(map[string]map[Topic]*handshake),
		deregisterFuncs:      make(map[Topic]func()),
	}
	api := &HandshakeAPI{
		namespace: "pss",
		ctrl:      ctrl,
	}
	pss.addAPI(rpc.API{
		Namespace: api.namespace,
		Version:   "0.2",
		Service:   api,
		Public:    true,
	})
	ctrlSingleton = ctrl
	return nil
}

//返回存储区中所有未过期的对称密钥
//对等（公钥）、主题和指定方向
func (ctl *HandshakeController) validKeys(pubkeyid string, topic *Topic, in bool) (validkeys []*string) {
	ctl.lock.Lock()
	defer ctl.lock.Unlock()
	now := time.Now()
	if _, ok := ctl.handshakes[pubkeyid]; !ok {
		return []*string{}
	} else if _, ok := ctl.handshakes[pubkeyid][*topic]; !ok {
		return []*string{}
	}
	var keystore *[]handshakeKey
	if in {
		keystore = &(ctl.handshakes[pubkeyid][*topic].inKeys)
	} else {
		keystore = &(ctl.handshakes[pubkeyid][*topic].outKeys)
	}

	for _, key := range *keystore {
		if key.limit <= key.count {
			ctl.releaseKey(*key.symKeyID, topic)
		} else if !key.expiredAt.IsZero() && key.expiredAt.Before(now) {
			ctl.releaseKey(*key.symKeyID, topic)
		} else {
			validkeys = append(validkeys, key.symKeyID)
		}
	}
	return
}

//将具有有效性限制的所有给定对称密钥添加到存储方式
//对等（公钥）、主题和指定方向
func (ctl *HandshakeController) updateKeys(pubkeyid string, topic *Topic, in bool, symkeyids []string, limit uint16) {
	ctl.lock.Lock()
	defer ctl.lock.Unlock()
	if _, ok := ctl.handshakes[pubkeyid]; !ok {
		ctl.handshakes[pubkeyid] = make(map[Topic]*handshake)

	}
	if ctl.handshakes[pubkeyid][*topic] == nil {
		ctl.handshakes[pubkeyid][*topic] = &handshake{}
	}
	var keystore *[]handshakeKey
	expire := time.Now()
	if in {
		keystore = &(ctl.handshakes[pubkeyid][*topic].inKeys)
	} else {
		keystore = &(ctl.handshakes[pubkeyid][*topic].outKeys)
		expire = expire.Add(time.Millisecond * ctl.symKeyExpiryTimeout)
	}
	for _, storekey := range *keystore {
		storekey.expiredAt = expire
	}
	for i := 0; i < len(symkeyids); i++ {
		storekey := handshakeKey{
			symKeyID: &symkeyids[i],
			pubKeyID: &pubkeyid,
			limit:    limit,
		}
		*keystore = append(*keystore, storekey)
		ctl.pss.symKeyPool[*storekey.symKeyID][*topic].protected = true
	}
	for i := 0; i < len(*keystore); i++ {
		ctl.symKeyIndex[*(*keystore)[i].symKeyID] = &((*keystore)[i])
	}
}

//使对称密钥过期，使其可用于垃圾收集
func (ctl *HandshakeController) releaseKey(symkeyid string, topic *Topic) bool {
	if ctl.symKeyIndex[symkeyid] == nil {
		log.Debug("no symkey", "symkeyid", symkeyid)
		return false
	}
	ctl.symKeyIndex[symkeyid].expiredAt = time.Now()
	log.Debug("handshake release", "symkeyid", symkeyid)
	return true
}

//检查给定方向上的所有对称密钥
//指定的对等机（公钥）和到期主题。
//过期手段：
//-设置了到期时间戳，超过了宽限期
//-已达到消息有效性限制
func (ctl *HandshakeController) cleanHandshake(pubkeyid string, topic *Topic, in bool, out bool) int {
	ctl.lock.Lock()
	defer ctl.lock.Unlock()
	var deletecount int
	var deletes []string
	now := time.Now()
	handshake := ctl.handshakes[pubkeyid][*topic]
	log.Debug("handshake clean", "pubkey", pubkeyid, "topic", topic)
	if in {
		for i, key := range handshake.inKeys {
			if key.expiredAt.Before(now) || (key.expiredAt.IsZero() && key.limit <= key.count) {
				log.Trace("handshake in clean remove", "symkeyid", *key.symKeyID)
				deletes = append(deletes, *key.symKeyID)
				handshake.inKeys[deletecount] = handshake.inKeys[i]
				deletecount++
			}
		}
		handshake.inKeys = handshake.inKeys[:len(handshake.inKeys)-deletecount]
	}
	if out {
		deletecount = 0
		for i, key := range handshake.outKeys {
			if key.expiredAt.Before(now) && (key.expiredAt.IsZero() && key.limit <= key.count) {
				log.Trace("handshake out clean remove", "symkeyid", *key.symKeyID)
				deletes = append(deletes, *key.symKeyID)
				handshake.outKeys[deletecount] = handshake.outKeys[i]
				deletecount++
			}
		}
		handshake.outKeys = handshake.outKeys[:len(handshake.outKeys)-deletecount]
	}
	for _, keyid := range deletes {
		delete(ctl.symKeyIndex, keyid)
		ctl.pss.symKeyPool[keyid][*topic].protected = false
	}
	return len(deletes)
}

//对所有对等端和主题运行cleanhandshake（）。
func (ctl *HandshakeController) clean() {
	peerpubkeys := ctl.handshakes
	for pubkeyid, peertopics := range peerpubkeys {
		for topic := range peertopics {
			ctl.cleanHandshake(pubkeyid, &topic, true, true)
		}
	}
}

//作为主题握手的pssmsg处理程序传递将在上激活
//处理传入的密钥交换消息和
//按对称密钥使用的CCUNTS消息（到期限制控制）
//仅当密钥处理程序失败时返回错误
func (ctl *HandshakeController) handler(msg []byte, p *p2p.Peer, asymmetric bool, symkeyid string) error {
	if !asymmetric {
		if ctl.symKeyIndex[symkeyid] != nil {
			if ctl.symKeyIndex[symkeyid].count >= ctl.symKeyIndex[symkeyid].limit {
				return fmt.Errorf("discarding message using expired key: %s", symkeyid)
			}
			ctl.symKeyIndex[symkeyid].count++
			log.Trace("increment symkey recv use", "symsymkeyid", symkeyid, "count", ctl.symKeyIndex[symkeyid].count, "limit", ctl.symKeyIndex[symkeyid].limit, "receiver", common.ToHex(crypto.FromECDSAPub(ctl.pss.PublicKey())))
		}
		return nil
	}
	keymsg := &handshakeMsg{}
	err := rlp.DecodeBytes(msg, keymsg)
	if err == nil {
		err := ctl.handleKeys(symkeyid, keymsg)
		if err != nil {
			log.Error("handlekeys fail", "error", err)
		}
		return err
	}
	return nil
}

//处理传入密钥交换消息
//将从对等端接收到的密钥添加到存储区
//生成并发送对等机请求的密钥数量
//
//TODO：
//防洪堤
//-键长检查
//-更新地址提示，如果：
//1）新地址中最左边的字节与存储的不匹配
//2）否则，如果新地址较长
func (ctl *HandshakeController) handleKeys(pubkeyid string, keymsg *handshakeMsg) error {
//来自对等机的新密钥
	if len(keymsg.Keys) > 0 {
		log.Debug("received handshake keys", "pubkeyid", pubkeyid, "from", keymsg.From, "count", len(keymsg.Keys))
		var sendsymkeyids []string
		for _, key := range keymsg.Keys {
			sendsymkey := make([]byte, len(key))
			copy(sendsymkey, key)
			sendsymkeyid, err := ctl.pss.setSymmetricKey(sendsymkey, keymsg.Topic, PssAddress(keymsg.From), false, false)
			if err != nil {
				return err
			}
			sendsymkeyids = append(sendsymkeyids, sendsymkeyid)
		}
		if len(sendsymkeyids) > 0 {
			ctl.updateKeys(pubkeyid, &keymsg.Topic, false, sendsymkeyids, keymsg.Limit)

			ctl.alertHandshake(pubkeyid, sendsymkeyids)
		}
	}

//对等密钥请求
	if keymsg.Request > 0 {
		_, err := ctl.sendKey(pubkeyid, &keymsg.Topic, keymsg.Request)
		if err != nil {
			return err
		}
	}

	return nil
}

//将密钥交换发送到对“topic”有效的对等方（公钥）
//将发送“keycount”指定的密钥数
//“msglmit”中指定的有效限制
//如果有效的传出密钥数小于理想/最大值
//金额，发送一个请求以获取要补足的密钥数量
//差异
func (ctl *HandshakeController) sendKey(pubkeyid string, topic *Topic, keycount uint8) ([]string, error) {

	var requestcount uint8
	to := PssAddress{}
	if _, ok := ctl.pss.pubKeyPool[pubkeyid]; !ok {
		return []string{}, errors.New("Invalid public key")
	} else if psp, ok := ctl.pss.pubKeyPool[pubkeyid][*topic]; ok {
		to = psp.address
	}

	recvkeys := make([][]byte, keycount)
	recvkeyids := make([]string, keycount)
	ctl.lock.Lock()
	if _, ok := ctl.handshakes[pubkeyid]; !ok {
		ctl.handshakes[pubkeyid] = make(map[Topic]*handshake)
	}
	ctl.lock.Unlock()

//检查缓冲区是否未满
	outkeys := ctl.validKeys(pubkeyid, topic, false)
	if len(outkeys) < int(ctl.symKeyCapacity) {
//请求计数=uint8（self.symkeycapacity-uint8（len（outkeys）））
		requestcount = ctl.symKeyCapacity
	}
//如果没有什么事要做就回来
	if requestcount == 0 && keycount == 0 {
		return []string{}, nil
	}

//生成要发送的新密钥
	for i := 0; i < len(recvkeyids); i++ {
		var err error
		recvkeyids[i], err = ctl.pss.GenerateSymmetricKey(*topic, to, true)
		if err != nil {
			return []string{}, fmt.Errorf("set receive symkey fail (pubkey %x topic %x): %v", pubkeyid, topic, err)
		}
		recvkeys[i], err = ctl.pss.GetSymmetricKey(recvkeyids[i])
		if err != nil {
			return []string{}, fmt.Errorf("GET Generated outgoing symkey fail (pubkey %x topic %x): %v", pubkeyid, topic, err)
		}
	}
	ctl.updateKeys(pubkeyid, topic, true, recvkeyids, ctl.symKeySendLimit)

//编码并发送消息
	recvkeymsg := &handshakeMsg{
		From:    ctl.pss.BaseAddr(),
		Keys:    recvkeys,
		Request: requestcount,
		Limit:   ctl.symKeySendLimit,
		Topic:   *topic,
	}
	log.Debug("sending our symkeys", "pubkey", pubkeyid, "symkeys", recvkeyids, "limit", ctl.symKeySendLimit, "requestcount", requestcount, "keycount", len(recvkeys))
	recvkeybytes, err := rlp.EncodeToBytes(recvkeymsg)
	if err != nil {
		return []string{}, fmt.Errorf("rlp keymsg encode fail: %v", err)
	}
//如果发送失败，则表示此公钥没有为此特定地址和主题注册。
	err = ctl.pss.SendAsym(pubkeyid, *topic, recvkeybytes)
	if err != nil {
		return []string{}, fmt.Errorf("Send symkey failed: %v", err)
	}
	return recvkeyids, nil
}

//对从密钥交换请求接收的密钥启用回调
func (ctl *HandshakeController) alertHandshake(pubkeyid string, symkeys []string) chan []string {
	if len(symkeys) > 0 {
		if _, ok := ctl.keyC[pubkeyid]; ok {
			ctl.keyC[pubkeyid] <- symkeys
			close(ctl.keyC[pubkeyid])
			delete(ctl.keyC, pubkeyid)
		}
		return nil
	}
	if _, ok := ctl.keyC[pubkeyid]; !ok {
		ctl.keyC[pubkeyid] = make(chan []string)
	}
	return ctl.keyC[pubkeyid]
}

type HandshakeAPI struct {
	namespace string
	ctrl      *HandshakeController
}

//为对等机（公钥）和主题启动握手会话
//组合。
//
//如果设置了“sync”，则调用将被阻止，直到从对等机收到密钥为止，
//或者如果握手请求超时
//
//如果设置了“flush”，将向对等机发送最大数量的密钥。
//不管存储区中当前存在多少有效密钥。
//
//返回可传递给pss.getSymmetricKey（）的对称密钥ID的列表
//用于检索对称密钥字节本身。
//
//如果传入的对称密钥存储已满（并且'flush'为false），则失败，
//或者如果基础密钥调度程序失败
func (api *HandshakeAPI) Handshake(pubkeyid string, topic Topic, sync bool, flush bool) (keys []string, err error) {
	var hsc chan []string
	var keycount uint8
	if flush {
		keycount = api.ctrl.symKeyCapacity
	} else {
		validkeys := api.ctrl.validKeys(pubkeyid, &topic, false)
		keycount = api.ctrl.symKeyCapacity - uint8(len(validkeys))
	}
	if keycount == 0 {
		return keys, errors.New("Incoming symmetric key store is already full")
	}
	if sync {
		hsc = api.ctrl.alertHandshake(pubkeyid, []string{})
	}
	_, err = api.ctrl.sendKey(pubkeyid, &topic, keycount)
	if err != nil {
		return keys, err
	}
	if sync {
		ctx, cancel := context.WithTimeout(context.Background(), api.ctrl.symKeyRequestTimeout)
		defer cancel()
		select {
		case keys = <-hsc:
			log.Trace("sync handshake response receive", "key", keys)
		case <-ctx.Done():
			return []string{}, errors.New("timeout")
		}
	}
	return keys, nil
}

//激活主题的握手功能
func (api *HandshakeAPI) AddHandshake(topic Topic) error {
	api.ctrl.deregisterFuncs[topic] = api.ctrl.pss.Register(&topic, NewHandler(api.ctrl.handler))
	return nil
}

//停用主题的握手功能
func (api *HandshakeAPI) RemoveHandshake(topic *Topic) error {
	if _, ok := api.ctrl.deregisterFuncs[*topic]; ok {
		api.ctrl.deregisterFuncs[*topic]()
	}
	return nil
}

//返回存储中每个对等机的所有有效对称密钥（公钥）
//话题。
//
//“in”和“out”参数指示哪个方向
//将返回对称密钥。
//如果两者都为假，则不会返回任何键（也不会返回任何错误）。
func (api *HandshakeAPI) GetHandshakeKeys(pubkeyid string, topic Topic, in bool, out bool) (keys []string, err error) {
	if in {
		for _, inkey := range api.ctrl.validKeys(pubkeyid, &topic, true) {
			keys = append(keys, *inkey)
		}
	}
	if out {
		for _, outkey := range api.ctrl.validKeys(pubkeyid, &topic, false) {
			keys = append(keys, *outkey)
		}
	}
	return keys, nil
}

//返回指定对称密钥的消息量
//在握手方案下仍然有效
func (api *HandshakeAPI) GetHandshakeKeyCapacity(symkeyid string) (uint16, error) {
	storekey := api.ctrl.symKeyIndex[symkeyid]
	if storekey == nil {
		return 0, fmt.Errorf("invalid symkey id %s", symkeyid)
	}
	return storekey.limit - storekey.count, nil
}

//返回公钥的字节表示形式（以ASCII十六进制表示）
//与给定的对称密钥关联
func (api *HandshakeAPI) GetHandshakePublicKey(symkeyid string) (string, error) {
	storekey := api.ctrl.symKeyIndex[symkeyid]
	if storekey == nil {
		return "", fmt.Errorf("invalid symkey id %s", symkeyid)
	}
	return *storekey.pubKeyID, nil
}

//手动终止给定的symkey
//
//如果设置了“flush”，则在返回之前将执行垃圾收集。
//
//成功删除时返回true，否则返回false
func (api *HandshakeAPI) ReleaseHandshakeKey(pubkeyid string, topic Topic, symkeyid string, flush bool) (removed bool, err error) {
	removed = api.ctrl.releaseKey(symkeyid, &topic)
	if removed && flush {
		api.ctrl.cleanHandshake(pubkeyid, &topic, true, true)
	}
	return
}

//在握手方案下发送对称消息
//
//重载pss.sendsym（）API调用，添加对称密钥使用计数
//用于消息到期控制
func (api *HandshakeAPI) SendSym(symkeyid string, topic Topic, msg hexutil.Bytes) (err error) {
	err = api.ctrl.pss.SendSym(symkeyid, topic, msg[:])
	if api.ctrl.symKeyIndex[symkeyid] != nil {
		if api.ctrl.symKeyIndex[symkeyid].count >= api.ctrl.symKeyIndex[symkeyid].limit {
			return errors.New("attempted send with expired key")
		}
		api.ctrl.symKeyIndex[symkeyid].count++
		log.Trace("increment symkey send use", "symkeyid", symkeyid, "count", api.ctrl.symKeyIndex[symkeyid].count, "limit", api.ctrl.symKeyIndex[symkeyid].limit, "receiver", common.ToHex(crypto.FromECDSAPub(api.ctrl.pss.PublicKey())))
	}
	return err
}
