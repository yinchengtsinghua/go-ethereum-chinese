
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

package pss

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/metrics/influxdb"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/pot"
	"github.com/ethereum/go-ethereum/swarm/state"
	whisper "github.com/ethereum/go-ethereum/whisper/whisperv5"
)

var (
	initOnce        = sync.Once{}
	loglevel        = flag.Int("loglevel", 2, "logging verbosity")
	longrunning     = flag.Bool("longrunning", false, "do run long-running tests")
	w               *whisper.Whisper
	wapi            *whisper.PublicWhisperAPI
	psslogmain      log.Logger
	pssprotocols    map[string]*protoCtrl
	useHandshake    bool
	noopHandlerFunc = func(msg []byte, p *p2p.Peer, asymmetric bool, keyid string) error {
		return nil
	}
)

func init() {
	flag.Parse()
	rand.Seed(time.Now().Unix())

	adapters.RegisterServices(newServices(false))
	initTest()
}

func initTest() {
	initOnce.Do(
		func() {
			psslogmain = log.New("psslog", "*")
			hs := log.StreamHandler(os.Stderr, log.TerminalFormat(true))
			hf := log.LvlFilterHandler(log.Lvl(*loglevel), hs)
			h := log.CallerFileHandler(hf)
			log.Root().SetHandler(h)

			w = whisper.New(&whisper.DefaultConfig)
			wapi = whisper.NewPublicWhisperAPI(w)

			pssprotocols = make(map[string]*protoCtrl)
		},
	)
}

//测试主题转换函数是否提供可预测的结果
func TestTopic(t *testing.T) {

	api := &API{}

	topicstr := strings.Join([]string{PingProtocol.Name, strconv.Itoa(int(PingProtocol.Version))}, ":")

//bytestotopic是权威的主题转换源
	topicobj := BytesToTopic([]byte(topicstr))

//主题字符串和主题字节必须匹配
	topicapiobj, _ := api.StringToTopic(topicstr)
	if topicobj != topicapiobj {
		t.Fatalf("bytes and string topic conversion mismatch; %s != %s", topicobj, topicapiobj)
	}

//topichex的字符串表示法
	topichex := topicobj.String()

//PingTopic上的ProtocolTopic包装应与TopicString相同
//检查是否匹配
	pingtopichex := PingTopic.String()
	if topichex != pingtopichex {
		t.Fatalf("protocol topic conversion mismatch; %s != %s", topichex, pingtopichex)
	}

//主题的json marshal
	topicjsonout, err := topicobj.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(topicjsonout)[1:len(topicjsonout)-1] != topichex {
		t.Fatalf("topic json marshal mismatch; %s != \"%s\"", topicjsonout, topichex)
	}

//主题JSON解组
	var topicjsonin Topic
	topicjsonin.UnmarshalJSON(topicjsonout)
	if topicjsonin != topicobj {
		t.Fatalf("topic json unmarshal mismatch: %x != %x", topicjsonin, topicobj)
	}
}

//消息控制标志的测试位打包
func TestMsgParams(t *testing.T) {
	var ctrl byte
	ctrl |= pssControlRaw
	p := newMsgParamsFromBytes([]byte{ctrl})
	m := newPssMsg(p)
	if !m.isRaw() || m.isSym() {
		t.Fatal("expected raw=true and sym=false")
	}
	ctrl |= pssControlSym
	p = newMsgParamsFromBytes([]byte{ctrl})
	m = newPssMsg(p)
	if !m.isRaw() || !m.isSym() {
		t.Fatal("expected raw=true and sym=true")
	}
	ctrl &= 0xff &^ pssControlRaw
	p = newMsgParamsFromBytes([]byte{ctrl})
	m = newPssMsg(p)
	if m.isRaw() || !m.isSym() {
		t.Fatal("expected raw=false and sym=true")
	}
}

//测试是否可以插入到缓存中，匹配具有缓存和缓存到期的项
func TestCache(t *testing.T) {
	var err error
	to, _ := hex.DecodeString("08090a0b0c0d0e0f1011121314150001020304050607161718191a1b1c1d1e1f")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	keys, err := wapi.NewKeyPair(ctx)
	privkey, err := w.GetPrivateKey(keys)
	if err != nil {
		t.Fatal(err)
	}
	ps := newTestPss(privkey, nil, nil)
	pp := NewPssParams().WithPrivateKey(privkey)
	data := []byte("foo")
	datatwo := []byte("bar")
	datathree := []byte("baz")
	wparams := &whisper.MessageParams{
		TTL:      defaultWhisperTTL,
		Src:      privkey,
		Dst:      &privkey.PublicKey,
		Topic:    whisper.TopicType(PingTopic),
		WorkTime: defaultWhisperWorkTime,
		PoW:      defaultWhisperPoW,
		Payload:  data,
	}
	woutmsg, err := whisper.NewSentMessage(wparams)
	env, err := woutmsg.Wrap(wparams)
	msg := &PssMsg{
		Payload: env,
		To:      to,
	}
	wparams.Payload = datatwo
	woutmsg, err = whisper.NewSentMessage(wparams)
	envtwo, err := woutmsg.Wrap(wparams)
	msgtwo := &PssMsg{
		Payload: envtwo,
		To:      to,
	}
	wparams.Payload = datathree
	woutmsg, err = whisper.NewSentMessage(wparams)
	envthree, err := woutmsg.Wrap(wparams)
	msgthree := &PssMsg{
		Payload: envthree,
		To:      to,
	}

	digest := ps.digest(msg)
	if err != nil {
		t.Fatalf("could not store cache msgone: %v", err)
	}
	digesttwo := ps.digest(msgtwo)
	if err != nil {
		t.Fatalf("could not store cache msgtwo: %v", err)
	}
	digestthree := ps.digest(msgthree)
	if err != nil {
		t.Fatalf("could not store cache msgthree: %v", err)
	}

	if digest == digesttwo {
		t.Fatalf("different msgs return same hash: %d", digesttwo)
	}

//检查缓存
	err = ps.addFwdCache(msg)
	if err != nil {
		t.Fatalf("write to pss expire cache failed: %v", err)
	}

	if !ps.checkFwdCache(msg) {
		t.Fatalf("message %v should have EXPIRE record in cache but checkCache returned false", msg)
	}

	if ps.checkFwdCache(msgtwo) {
		t.Fatalf("message %v should NOT have EXPIRE record in cache but checkCache returned true", msgtwo)
	}

	time.Sleep(pp.CacheTTL + 1*time.Second)
	err = ps.addFwdCache(msgthree)
	if err != nil {
		t.Fatalf("write to pss expire cache failed: %v", err)
	}

	if ps.checkFwdCache(msg) {
		t.Fatalf("message %v should have expired from cache but checkCache returned true", msg)
	}

	if _, ok := ps.fwdCache[digestthree]; !ok {
		t.Fatalf("unexpired message should be in the cache: %v", digestthree)
	}

	if _, ok := ps.fwdCache[digesttwo]; ok {
		t.Fatalf("expired message should have been cleared from the cache: %v", digesttwo)
	}
}

//地址提示的匹配；消息是否可以是节点的
func TestAddressMatch(t *testing.T) {

	localaddr := network.RandomAddr().Over()
	copy(localaddr[:8], []byte("deadbeef"))
	remoteaddr := []byte("feedbeef")
	kadparams := network.NewKadParams()
	kad := network.NewKademlia(localaddr, kadparams)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	keys, err := wapi.NewKeyPair(ctx)
	if err != nil {
		t.Fatalf("Could not generate private key: %v", err)
	}
	privkey, err := w.GetPrivateKey(keys)
	pssp := NewPssParams().WithPrivateKey(privkey)
	ps, err := NewPss(kad, pssp)
	if err != nil {
		t.Fatal(err.Error())
	}

	pssmsg := &PssMsg{
		To: remoteaddr,
	}

//与第一个字节不同
	if ps.isSelfRecipient(pssmsg) {
		t.Fatalf("isSelfRecipient true but %x != %x", remoteaddr, localaddr)
	}
	if ps.isSelfPossibleRecipient(pssmsg, false) {
		t.Fatalf("isSelfPossibleRecipient true but %x != %x", remoteaddr[:8], localaddr[:8])
	}

//8个前字节相同
	copy(remoteaddr[:4], localaddr[:4])
	if ps.isSelfRecipient(pssmsg) {
		t.Fatalf("isSelfRecipient true but %x != %x", remoteaddr, localaddr)
	}
	if !ps.isSelfPossibleRecipient(pssmsg, false) {
		t.Fatalf("isSelfPossibleRecipient false but %x == %x", remoteaddr[:8], localaddr[:8])
	}

//所有字节相同
	pssmsg.To = localaddr
	if !ps.isSelfRecipient(pssmsg) {
		t.Fatalf("isSelfRecipient false but %x == %x", remoteaddr, localaddr)
	}
	if !ps.isSelfPossibleRecipient(pssmsg, false) {
		t.Fatalf("isSelfPossibleRecipient false but %x == %x", remoteaddr[:8], localaddr[:8])
	}

}

//如果代理处理程序存在且发送方位于消息代理中，测试发送方是否处理消息。
func TestProxShortCircuit(t *testing.T) {

//发送方节点地址
	localAddr := network.RandomAddr().Over()
	localPotAddr := pot.NewAddressFromBytes(localAddr)

//成立卡德利亚
	kadParams := network.NewKadParams()
	kad := network.NewKademlia(localAddr, kadParams)
	peerCount := kad.MinBinSize + 1

//设置PSS
	privKey, err := crypto.GenerateKey()
	pssp := NewPssParams().WithPrivateKey(privKey)
	ps, err := NewPss(kad, pssp)
	if err != nil {
		t.Fatal(err.Error())
	}

//创建Kademlia对等点，这样我们就有了minproxlimit内外的对等点。
	var peers []*network.Peer
	proxMessageAddress := pot.RandomAddressAt(localPotAddr, peerCount).Bytes()
	distantMessageAddress := pot.RandomAddressAt(localPotAddr, 0).Bytes()

	for i := 0; i < peerCount; i++ {
		rw := &p2p.MsgPipeRW{}
		ptpPeer := p2p.NewPeer(enode.ID{}, "wanna be with me? [ ] yes [ ] no", []p2p.Cap{})
		protoPeer := protocols.NewPeer(ptpPeer, rw, &protocols.Spec{})
		peerAddr := pot.RandomAddressAt(localPotAddr, i)
		bzzPeer := &network.BzzPeer{
			Peer: protoPeer,
			BzzAddr: &network.BzzAddr{
				OAddr: peerAddr.Bytes(),
				UAddr: []byte(fmt.Sprintf("%x", peerAddr[:])),
			},
		}
		peer := network.NewPeer(bzzPeer, kad)
		kad.On(peer)
		peers = append(peers, peer)
	}

//注册IT标记代理功能
	delivered := make(chan struct{})
	rawHandlerFunc := func(msg []byte, p *p2p.Peer, asymmetric bool, keyid string) error {
		log.Trace("in allowraw handler")
		delivered <- struct{}{}
		return nil
	}
	topic := BytesToTopic([]byte{0x2a})
	hndlrProxDereg := ps.Register(&topic, &handler{
		f: rawHandlerFunc,
		caps: &handlerCaps{
			raw:  true,
			prox: true,
		},
	})
	defer hndlrProxDereg()

//发送消息太远，发件人不在代理中
//接收此消息应超时
	errC := make(chan error)
	go func() {
		err := ps.SendRaw(distantMessageAddress, topic, []byte("foo"))
		if err != nil {
			errC <- err
		}
	}()

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	defer cancel()
	select {
	case <-delivered:
		t.Fatal("raw distant message delivered")
	case err := <-errC:
		t.Fatal(err)
	case <-ctx.Done():
	}

//发送应在发送方代理内的消息
//应传递此消息
	go func() {
		err := ps.SendRaw(proxMessageAddress, topic, []byte("bar"))
		if err != nil {
			errC <- err
		}
	}()

	ctx, cancel = context.WithTimeout(context.TODO(), time.Second)
	defer cancel()
	select {
	case <-delivered:
	case err := <-errC:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("raw timeout")
	}

//使用sym和asym send尝试相同的prox消息
	proxAddrPss := PssAddress(proxMessageAddress)
	symKeyId, err := ps.GenerateSymmetricKey(topic, proxAddrPss, true)
	go func() {
		err := ps.SendSym(symKeyId, topic, []byte("baz"))
		if err != nil {
			errC <- err
		}
	}()
	ctx, cancel = context.WithTimeout(context.TODO(), time.Second)
	defer cancel()
	select {
	case <-delivered:
	case err := <-errC:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("sym timeout")
	}

	err = ps.SetPeerPublicKey(&privKey.PublicKey, topic, proxAddrPss)
	if err != nil {
		t.Fatal(err)
	}
	pubKeyId := hexutil.Encode(crypto.FromECDSAPub(&privKey.PublicKey))
	go func() {
		err := ps.SendAsym(pubKeyId, topic, []byte("xyzzy"))
		if err != nil {
			errC <- err
		}
	}()
	ctx, cancel = context.WithTimeout(context.TODO(), time.Second)
	defer cancel()
	select {
	case <-delivered:
	case err := <-errC:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("asym timeout")
	}
}

//验证是否可以将节点设置为收件人，无论显式消息地址是否匹配，如果主题的至少一个处理程序被显式设置为允许它
//注意，在这些测试中，为了方便起见，我们在处理程序上使用原始功能
func TestAddressMatchProx(t *testing.T) {

//收件人节点地址
	localAddr := network.RandomAddr().Over()
	localPotAddr := pot.NewAddressFromBytes(localAddr)

//成立卡德利亚
	kadparams := network.NewKadParams()
	kad := network.NewKademlia(localAddr, kadparams)
	nnPeerCount := kad.MinBinSize
	peerCount := nnPeerCount + 2

//设置PSS
	privKey, err := crypto.GenerateKey()
	pssp := NewPssParams().WithPrivateKey(privKey)
	ps, err := NewPss(kad, pssp)
	if err != nil {
		t.Fatal(err.Error())
	}

//创建Kademlia对等点，这样我们就有了minproxlimit内外的对等点。
	var peers []*network.Peer
	for i := 0; i < peerCount; i++ {
		rw := &p2p.MsgPipeRW{}
		ptpPeer := p2p.NewPeer(enode.ID{}, "362436 call me anytime", []p2p.Cap{})
		protoPeer := protocols.NewPeer(ptpPeer, rw, &protocols.Spec{})
		peerAddr := pot.RandomAddressAt(localPotAddr, i)
		bzzPeer := &network.BzzPeer{
			Peer: protoPeer,
			BzzAddr: &network.BzzAddr{
				OAddr: peerAddr.Bytes(),
				UAddr: []byte(fmt.Sprintf("%x", peerAddr[:])),
			},
		}
		peer := network.NewPeer(bzzPeer, kad)
		kad.On(peer)
		peers = append(peers, peer)
	}

//TODO:在网络包中创建一个测试，以生成一个具有n个对等机的表，其中n-m是代理对等机
//同时对Kademlia进行测试回归，因为我们正在从不同的包中编译测试参数。
	var proxes int
	var conns int
	depth := kad.NeighbourhoodDepth()
	kad.EachConn(nil, peerCount, func(p *network.Peer, po int) bool {
		conns++
		if po >= depth {
			proxes++
		}
		return true
	})
	if proxes != nnPeerCount {
		t.Fatalf("expected %d proxpeers, have %d", nnPeerCount, proxes)
	} else if conns != peerCount {
		t.Fatalf("expected %d peers total, have %d", peerCount, proxes)
	}

//从localaddr到try的远程地址距离以及使用prox处理程序时的预期结果
	remoteDistances := []int{
		255,
		nnPeerCount + 1,
		nnPeerCount,
		nnPeerCount - 1,
		0,
	}
	expects := []bool{
		true,
		true,
		true,
		false,
		false,
	}

//首先对使用prox计算可能接收的方法进行单元测试
	for i, distance := range remoteDistances {
		pssMsg := newPssMsg(&msgParams{})
		pssMsg.To = make([]byte, len(localAddr))
		copy(pssMsg.To, localAddr)
		var byteIdx = distance / 8
		pssMsg.To[byteIdx] ^= 1 << uint(7-(distance%8))
		log.Trace(fmt.Sprintf("addrmatch %v", bytes.Equal(pssMsg.To, localAddr)))
		if ps.isSelfPossibleRecipient(pssMsg, true) != expects[i] {
			t.Fatalf("expected distance %d to be %v", distance, expects[i])
		}
	}

//我们向上移动到更高的级别并测试实际的消息处理程序
//对于每个距离，检查在使用prox变量时，我们是否可能是接收者。

//此处理程序将为传递给处理程序的每个消息增加一个计数器。
	var receives int
	rawHandlerFunc := func(msg []byte, p *p2p.Peer, asymmetric bool, keyid string) error {
		log.Trace("in allowraw handler")
		receives++
		return nil
	}

//注册IT标记代理功能
	topic := BytesToTopic([]byte{0x2a})
	hndlrProxDereg := ps.Register(&topic, &handler{
		f: rawHandlerFunc,
		caps: &handlerCaps{
			raw:  true,
			prox: true,
		},
	})

//测试距离
	var prevReceive int
	for i, distance := range remoteDistances {
		remotePotAddr := pot.RandomAddressAt(localPotAddr, distance)
		remoteAddr := remotePotAddr.Bytes()

		var data [32]byte
		rand.Read(data[:])
		pssMsg := newPssMsg(&msgParams{raw: true})
		pssMsg.To = remoteAddr
		pssMsg.Expire = uint32(time.Now().Unix() + 4200)
		pssMsg.Payload = &whisper.Envelope{
			Topic: whisper.TopicType(topic),
			Data:  data[:],
		}

		log.Trace("withprox addrs", "local", localAddr, "remote", remoteAddr)
		ps.handlePssMsg(context.TODO(), pssMsg)
		if (!expects[i] && prevReceive != receives) || (expects[i] && prevReceive == receives) {
			t.Fatalf("expected distance %d recipient %v when prox is set for handler", distance, expects[i])
		}
		prevReceive = receives
	}

//现在添加一个不支持代理的处理程序并测试
	ps.Register(&topic, &handler{
		f: rawHandlerFunc,
		caps: &handlerCaps{
			raw: true,
		},
	})
	receives = 0
	prevReceive = 0
	for i, distance := range remoteDistances {
		remotePotAddr := pot.RandomAddressAt(localPotAddr, distance)
		remoteAddr := remotePotAddr.Bytes()

		var data [32]byte
		rand.Read(data[:])
		pssMsg := newPssMsg(&msgParams{raw: true})
		pssMsg.To = remoteAddr
		pssMsg.Expire = uint32(time.Now().Unix() + 4200)
		pssMsg.Payload = &whisper.Envelope{
			Topic: whisper.TopicType(topic),
			Data:  data[:],
		}

		log.Trace("withprox addrs", "local", localAddr, "remote", remoteAddr)
		ps.handlePssMsg(context.TODO(), pssMsg)
		if (!expects[i] && prevReceive != receives) || (expects[i] && prevReceive == receives) {
			t.Fatalf("expected distance %d recipient %v when prox is set for handler", distance, expects[i])
		}
		prevReceive = receives
	}

//现在取消注册支持代理的处理程序，现在不会处理任何消息
	hndlrProxDereg()
	receives = 0

	for _, distance := range remoteDistances {
		remotePotAddr := pot.RandomAddressAt(localPotAddr, distance)
		remoteAddr := remotePotAddr.Bytes()

		pssMsg := newPssMsg(&msgParams{raw: true})
		pssMsg.To = remoteAddr
		pssMsg.Expire = uint32(time.Now().Unix() + 4200)
		pssMsg.Payload = &whisper.Envelope{
			Topic: whisper.TopicType(topic),
			Data:  []byte(remotePotAddr.String()),
		}

		log.Trace("noprox addrs", "local", localAddr, "remote", remoteAddr)
		ps.handlePssMsg(context.TODO(), pssMsg)
		if receives != 0 {
			t.Fatalf("expected distance %d to not be recipient when prox is not set for handler", distance)
		}

	}
}

//验证消息队列是否在应该的时候发生，以及是否删除过期和损坏的消息
func TestMessageProcessing(t *testing.T) {

	t.Skip("Disabled due to probable faulty logic for outbox expectations")
//设置
	privkey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err.Error())
	}

	addr := make([]byte, 32)
	addr[0] = 0x01
	ps := newTestPss(privkey, network.NewKademlia(addr, network.NewKadParams()), NewPssParams())

//消息应该通过
	msg := newPssMsg(&msgParams{})
	msg.To = addr
	msg.Expire = uint32(time.Now().Add(time.Second * 60).Unix())
	msg.Payload = &whisper.Envelope{
		Topic: [4]byte{},
		Data:  []byte{0x66, 0x6f, 0x6f},
	}
	if err := ps.handlePssMsg(context.TODO(), msg); err != nil {
		t.Fatal(err.Error())
	}
	tmr := time.NewTimer(time.Millisecond * 100)
	var outmsg *PssMsg
	select {
	case outmsg = <-ps.outbox:
	case <-tmr.C:
	default:
	}
	if outmsg != nil {
		t.Fatalf("expected outbox empty after full address on msg, but had message %s", msg)
	}

//由于部分长度，消息应通过并排队
	msg.To = addr[0:1]
	msg.Payload.Data = []byte{0x78, 0x79, 0x80, 0x80, 0x79}
	if err := ps.handlePssMsg(context.TODO(), msg); err != nil {
		t.Fatal(err.Error())
	}
	tmr.Reset(time.Millisecond * 100)
	outmsg = nil
	select {
	case outmsg = <-ps.outbox:
	case <-tmr.C:
	}
	if outmsg == nil {
		t.Fatal("expected message in outbox on encrypt fail, but empty")
	}
	outmsg = nil
	select {
	case outmsg = <-ps.outbox:
	default:
	}
	if outmsg != nil {
		t.Fatalf("expected only one queued message but also had message %v", msg)
	}

//完全地址不匹配应将消息放入队列
	msg.To[0] = 0xff
	if err := ps.handlePssMsg(context.TODO(), msg); err != nil {
		t.Fatal(err.Error())
	}
	tmr.Reset(time.Millisecond * 10)
	outmsg = nil
	select {
	case outmsg = <-ps.outbox:
	case <-tmr.C:
	}
	if outmsg == nil {
		t.Fatal("expected message in outbox on address mismatch, but empty")
	}
	outmsg = nil
	select {
	case outmsg = <-ps.outbox:
	default:
	}
	if outmsg != nil {
		t.Fatalf("expected only one queued message but also had message %v", msg)
	}

//应删除过期的邮件
	msg.Expire = uint32(time.Now().Add(-time.Second).Unix())
	if err := ps.handlePssMsg(context.TODO(), msg); err != nil {
		t.Fatal(err.Error())
	}
	tmr.Reset(time.Millisecond * 10)
	outmsg = nil
	select {
	case outmsg = <-ps.outbox:
	case <-tmr.C:
	default:
	}
	if outmsg != nil {
		t.Fatalf("expected empty queue but have message %v", msg)
	}

//无效消息应返回错误
	fckedupmsg := &struct {
		pssMsg *PssMsg
	}{
		pssMsg: &PssMsg{},
	}
	if err := ps.handlePssMsg(context.TODO(), fckedupmsg); err == nil {
		t.Fatalf("expected error from processMsg but error nil")
	}

//发件箱已满应返回错误
	msg.Expire = uint32(time.Now().Add(time.Second * 60).Unix())
	for i := 0; i < defaultOutboxCapacity; i++ {
		ps.outbox <- msg
	}
	msg.Payload.Data = []byte{0x62, 0x61, 0x72}
	err = ps.handlePssMsg(context.TODO(), msg)
	if err == nil {
		t.Fatal("expected error when mailbox full, but was nil")
	}
}

//设置和生成公钥和符号键
func TestKeys(t *testing.T) {
//制作我们的密钥并用它初始化PSS
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ourkeys, err := wapi.NewKeyPair(ctx)
	if err != nil {
		t.Fatalf("create 'our' key fail")
	}
	ctx, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	theirkeys, err := wapi.NewKeyPair(ctx)
	if err != nil {
		t.Fatalf("create 'their' key fail")
	}
	ourprivkey, err := w.GetPrivateKey(ourkeys)
	if err != nil {
		t.Fatalf("failed to retrieve 'our' private key")
	}
	theirprivkey, err := w.GetPrivateKey(theirkeys)
	if err != nil {
		t.Fatalf("failed to retrieve 'their' private key")
	}
	ps := newTestPss(ourprivkey, nil, nil)

//使用模拟地址、映射到模拟公共地址和模拟符号密钥设置对等机
	addr := make(PssAddress, 32)
	copy(addr, network.RandomAddr().Over())
	outkey := network.RandomAddr().Over()
	topicobj := BytesToTopic([]byte("foo:42"))
	ps.SetPeerPublicKey(&theirprivkey.PublicKey, topicobj, addr)
	outkeyid, err := ps.SetSymmetricKey(outkey, topicobj, addr, false)
	if err != nil {
		t.Fatalf("failed to set 'our' outgoing symmetric key")
	}

//生成一个对称密钥，我们将向对等机发送该密钥，以便对发送给我们的消息进行加密。
	inkeyid, err := ps.GenerateSymmetricKey(topicobj, addr, true)
	if err != nil {
		t.Fatalf("failed to set 'our' incoming symmetric key")
	}

//把钥匙从耳语中拿回来，检查它是否仍然一样
	outkeyback, err := ps.w.GetSymKey(outkeyid)
	if err != nil {
		t.Fatalf(err.Error())
	}
	inkey, err := ps.w.GetSymKey(inkeyid)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if !bytes.Equal(outkeyback, outkey) {
		t.Fatalf("passed outgoing symkey doesnt equal stored: %x / %x", outkey, outkeyback)
	}

	t.Logf("symout: %v", outkeyback)
	t.Logf("symin: %v", inkey)

//检查密钥是否存储在对等池中
	psp := ps.symKeyPool[inkeyid][topicobj]
	if !bytes.Equal(psp.address, addr) {
		t.Fatalf("inkey address does not match; %p != %p", psp.address, addr)
	}
}

//检查我们是否可以为每个主题和对等方检索以前添加的公钥实体
func TestGetPublickeyEntries(t *testing.T) {

	privkey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	ps := newTestPss(privkey, nil, nil)

	peeraddr := network.RandomAddr().Over()
	topicaddr := make(map[Topic]PssAddress)
	topicaddr[Topic{0x13}] = peeraddr
	topicaddr[Topic{0x2a}] = peeraddr[:16]
	topicaddr[Topic{0x02, 0x9a}] = []byte{}

	remoteprivkey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	remotepubkeybytes := crypto.FromECDSAPub(&remoteprivkey.PublicKey)
	remotepubkeyhex := common.ToHex(remotepubkeybytes)

	pssapi := NewAPI(ps)

	for to, a := range topicaddr {
		err = pssapi.SetPeerPublicKey(remotepubkeybytes, to, a)
		if err != nil {
			t.Fatal(err)
		}
	}

	intopic, err := pssapi.GetPeerTopics(remotepubkeyhex)
	if err != nil {
		t.Fatal(err)
	}

OUTER:
	for _, tnew := range intopic {
		for torig, addr := range topicaddr {
			if bytes.Equal(torig[:], tnew[:]) {
				inaddr, err := pssapi.GetPeerAddress(remotepubkeyhex, torig)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(addr, inaddr) {
					t.Fatalf("Address mismatch for topic %x; got %x, expected %x", torig, inaddr, addr)
				}
				delete(topicaddr, torig)
				continue OUTER
			}
		}
		t.Fatalf("received topic %x did not match any existing topics", tnew)
	}

	if len(topicaddr) != 0 {
		t.Fatalf("%d topics were not matched", len(topicaddr))
	}
}

//转发应跳过没有匹配PSS功能的对等端
func TestPeerCapabilityMismatch(t *testing.T) {

//为转发器节点创建私钥
	privkey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

//初始化KAD
	baseaddr := network.RandomAddr()
	kad := network.NewKademlia((baseaddr).Over(), network.NewKadParams())
	rw := &p2p.MsgPipeRW{}

//一个对等机的PSS版本不匹配
	wrongpssaddr := network.RandomAddr()
	wrongpsscap := p2p.Cap{
		Name:    pssProtocolName,
		Version: 0,
	}
	nid := enode.ID{0x01}
	wrongpsspeer := network.NewPeer(&network.BzzPeer{
		Peer:    protocols.NewPeer(p2p.NewPeer(nid, common.ToHex(wrongpssaddr.Over()), []p2p.Cap{wrongpsscap}), rw, nil),
		BzzAddr: &network.BzzAddr{OAddr: wrongpssaddr.Over(), UAddr: nil},
	}, kad)

//一个同伴甚至没有PSS（boo！）
	nopssaddr := network.RandomAddr()
	nopsscap := p2p.Cap{
		Name:    "nopss",
		Version: 1,
	}
	nid = enode.ID{0x02}
	nopsspeer := network.NewPeer(&network.BzzPeer{
		Peer:    protocols.NewPeer(p2p.NewPeer(nid, common.ToHex(nopssaddr.Over()), []p2p.Cap{nopsscap}), rw, nil),
		BzzAddr: &network.BzzAddr{OAddr: nopssaddr.Over(), UAddr: nil},
	}, kad)

//将对等点添加到Kademlia并激活它们
//它是安全的，所以不要检查错误
	kad.Register(wrongpsspeer.BzzAddr)
	kad.On(wrongpsspeer)
	kad.Register(nopsspeer.BzzAddr)
	kad.On(nopsspeer)

//创建PSS
	pssmsg := &PssMsg{
		To:      []byte{},
		Expire:  uint32(time.Now().Add(time.Second).Unix()),
		Payload: &whisper.Envelope{},
	}
	ps := newTestPss(privkey, kad, nil)

//向前跑
//这就足够完成了；试图发送给不具备能力的对等方将创建segfault
	ps.forward(pssmsg)

}

//验证仅当存在主题的至少一个处理程序（其中显式允许原始消息）时才调用原始消息的消息处理程序
func TestRawAllow(t *testing.T) {

//像以前那样多次设置PSS
	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	baseAddr := network.RandomAddr()
	kad := network.NewKademlia((baseAddr).Over(), network.NewKadParams())
	ps := newTestPss(privKey, kad, nil)
	topic := BytesToTopic([]byte{0x2a})

//创建处理程序的内部目录，该处理程序在每次消息命中它时递增
	var receives int
	rawHandlerFunc := func(msg []byte, p *p2p.Peer, asymmetric bool, keyid string) error {
		log.Trace("in allowraw handler")
		receives++
		return nil
	}

//用不带原始功能的处理程序包装此处理程序函数并注册它
	hndlrNoRaw := &handler{
		f: rawHandlerFunc,
	}
	ps.Register(&topic, hndlrNoRaw)

//用原始消息测试它，应该是poo poo
	pssMsg := newPssMsg(&msgParams{
		raw: true,
	})
	pssMsg.To = baseAddr.OAddr
	pssMsg.Expire = uint32(time.Now().Unix() + 4200)
	pssMsg.Payload = &whisper.Envelope{
		Topic: whisper.TopicType(topic),
	}
	ps.handlePssMsg(context.TODO(), pssMsg)
	if receives > 0 {
		t.Fatalf("Expected handler not to be executed with raw cap off")
	}

//现在用原始功能包装相同的处理程序函数并注册它
	hndlrRaw := &handler{
		f: rawHandlerFunc,
		caps: &handlerCaps{
			raw: true,
		},
	}
	deregRawHandler := ps.Register(&topic, hndlrRaw)

//现在应该工作
	pssMsg.Payload.Data = []byte("Raw Deal")
	ps.handlePssMsg(context.TODO(), pssMsg)
	if receives == 0 {
		t.Fatalf("Expected handler to be executed with raw cap on")
	}

//现在注销具有原始功能的处理程序
	prevReceives := receives
	deregRawHandler()

//检查原始消息是否再次失败
	pssMsg.Payload.Data = []byte("Raw Trump")
	ps.handlePssMsg(context.TODO(), pssMsg)
	if receives != prevReceives {
		t.Fatalf("Expected handler not to be executed when raw handler is retracted")
	}
}

//下面是使用模拟框架的测试

//测试API层是否可以处理边缘大小写值
func TestApi(t *testing.T) {
	clients, err := setupNetwork(2, true)
	if err != nil {
		t.Fatal(err)
	}

	topic := "0xdeadbeef"

	err = clients[0].Call(nil, "pss_sendRaw", "0x", topic, "0x666f6f")
	if err != nil {
		t.Fatal(err)
	}

	err = clients[0].Call(nil, "pss_sendRaw", "0xabcdef", topic, "0x")
	if err == nil {
		t.Fatal("expected error on empty msg")
	}

	overflowAddr := [33]byte{}
	err = clients[0].Call(nil, "pss_sendRaw", hexutil.Encode(overflowAddr[:]), topic, "0x666f6f")
	if err == nil {
		t.Fatal("expected error on send too big address")
	}
}

//验证节点是否可以发送和接收原始（逐字）消息
func TestSendRaw(t *testing.T) {
	t.Run("32", testSendRaw)
	t.Run("8", testSendRaw)
	t.Run("0", testSendRaw)
}

func testSendRaw(t *testing.T) {

	var addrsize int64
	var err error

	paramstring := strings.Split(t.Name(), "/")

	addrsize, _ = strconv.ParseInt(paramstring[1], 10, 0)
	log.Info("raw send test", "addrsize", addrsize)

	clients, err := setupNetwork(2, true)
	if err != nil {
		t.Fatal(err)
	}

	topic := "0xdeadbeef"

	var loaddrhex string
	err = clients[0].Call(&loaddrhex, "pss_baseAddr")
	if err != nil {
		t.Fatalf("rpc get node 1 baseaddr fail: %v", err)
	}
	loaddrhex = loaddrhex[:2+(addrsize*2)]
	var roaddrhex string
	err = clients[1].Call(&roaddrhex, "pss_baseAddr")
	if err != nil {
		t.Fatalf("rpc get node 2 baseaddr fail: %v", err)
	}
	roaddrhex = roaddrhex[:2+(addrsize*2)]

	time.Sleep(time.Millisecond * 500)

//此时，我们已经验证了在每个对等机上保存和匹配符号键。
//现在尝试向两个方向发送对称加密的消息
	lmsgC := make(chan APIMsg)
	lctx, lcancel := context.WithTimeout(context.Background(), time.Second*10)
	defer lcancel()
	lsub, err := clients[0].Subscribe(lctx, "pss", lmsgC, "receive", topic, true, false)
	log.Trace("lsub", "id", lsub)
	defer lsub.Unsubscribe()
	rmsgC := make(chan APIMsg)
	rctx, rcancel := context.WithTimeout(context.Background(), time.Second*10)
	defer rcancel()
	rsub, err := clients[1].Subscribe(rctx, "pss", rmsgC, "receive", topic, true, false)
	log.Trace("rsub", "id", rsub)
	defer rsub.Unsubscribe()

//发送并验证传递
	lmsg := []byte("plugh")
	err = clients[1].Call(nil, "pss_sendRaw", loaddrhex, topic, hexutil.Encode(lmsg))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case recvmsg := <-lmsgC:
		if !bytes.Equal(recvmsg.Msg, lmsg) {
			t.Fatalf("node 1 received payload mismatch: expected %v, got %v", lmsg, recvmsg)
		}
	case cerr := <-lctx.Done():
		t.Fatalf("test message (left) timed out: %v", cerr)
	}
	rmsg := []byte("xyzzy")
	err = clients[0].Call(nil, "pss_sendRaw", roaddrhex, topic, hexutil.Encode(rmsg))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case recvmsg := <-rmsgC:
		if !bytes.Equal(recvmsg.Msg, rmsg) {
			t.Fatalf("node 2 received payload mismatch: expected %x, got %v", rmsg, recvmsg.Msg)
		}
	case cerr := <-rctx.Done():
		t.Fatalf("test message (right) timed out: %v", cerr)
	}
}

//在两个直接连接的对等端之间发送对称加密的消息
func TestSendSym(t *testing.T) {
	t.Run("32", testSendSym)
	t.Run("8", testSendSym)
	t.Run("0", testSendSym)
}

func testSendSym(t *testing.T) {

//地址提示大小
	var addrsize int64
	var err error
	paramstring := strings.Split(t.Name(), "/")
	addrsize, _ = strconv.ParseInt(paramstring[1], 10, 0)
	log.Info("sym send test", "addrsize", addrsize)

	clients, err := setupNetwork(2, false)
	if err != nil {
		t.Fatal(err)
	}

	var topic string
	err = clients[0].Call(&topic, "pss_stringToTopic", "foo:42")
	if err != nil {
		t.Fatal(err)
	}

	var loaddrhex string
	err = clients[0].Call(&loaddrhex, "pss_baseAddr")
	if err != nil {
		t.Fatalf("rpc get node 1 baseaddr fail: %v", err)
	}
	loaddrhex = loaddrhex[:2+(addrsize*2)]
	var roaddrhex string
	err = clients[1].Call(&roaddrhex, "pss_baseAddr")
	if err != nil {
		t.Fatalf("rpc get node 2 baseaddr fail: %v", err)
	}
	roaddrhex = roaddrhex[:2+(addrsize*2)]

//从PSS实例检索公钥
//互惠设置此公钥
	var lpubkeyhex string
	err = clients[0].Call(&lpubkeyhex, "pss_getPublicKey")
	if err != nil {
		t.Fatalf("rpc get node 1 pubkey fail: %v", err)
	}
	var rpubkeyhex string
	err = clients[1].Call(&rpubkeyhex, "pss_getPublicKey")
	if err != nil {
		t.Fatalf("rpc get node 2 pubkey fail: %v", err)
	}

	time.Sleep(time.Millisecond * 500)

//此时，我们已经验证了在每个对等机上保存和匹配符号键。
//现在尝试向两个方向发送对称加密的消息
	lmsgC := make(chan APIMsg)
	lctx, lcancel := context.WithTimeout(context.Background(), time.Second*10)
	defer lcancel()
	lsub, err := clients[0].Subscribe(lctx, "pss", lmsgC, "receive", topic, false, false)
	log.Trace("lsub", "id", lsub)
	defer lsub.Unsubscribe()
	rmsgC := make(chan APIMsg)
	rctx, rcancel := context.WithTimeout(context.Background(), time.Second*10)
	defer rcancel()
	rsub, err := clients[1].Subscribe(rctx, "pss", rmsgC, "receive", topic, false, false)
	log.Trace("rsub", "id", rsub)
	defer rsub.Unsubscribe()

	lrecvkey := network.RandomAddr().Over()
	rrecvkey := network.RandomAddr().Over()

	var lkeyids [2]string
	var rkeyids [2]string

//手动设置互惠符号键
	err = clients[0].Call(&lkeyids, "psstest_setSymKeys", rpubkeyhex, lrecvkey, rrecvkey, defaultSymKeySendLimit, topic, roaddrhex)
	if err != nil {
		t.Fatal(err)
	}
	err = clients[1].Call(&rkeyids, "psstest_setSymKeys", lpubkeyhex, rrecvkey, lrecvkey, defaultSymKeySendLimit, topic, loaddrhex)
	if err != nil {
		t.Fatal(err)
	}

//发送并验证传递
	lmsg := []byte("plugh")
	err = clients[1].Call(nil, "pss_sendSym", rkeyids[1], topic, hexutil.Encode(lmsg))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case recvmsg := <-lmsgC:
		if !bytes.Equal(recvmsg.Msg, lmsg) {
			t.Fatalf("node 1 received payload mismatch: expected %v, got %v", lmsg, recvmsg)
		}
	case cerr := <-lctx.Done():
		t.Fatalf("test message timed out: %v", cerr)
	}
	rmsg := []byte("xyzzy")
	err = clients[0].Call(nil, "pss_sendSym", lkeyids[1], topic, hexutil.Encode(rmsg))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case recvmsg := <-rmsgC:
		if !bytes.Equal(recvmsg.Msg, rmsg) {
			t.Fatalf("node 2 received payload mismatch: expected %x, got %v", rmsg, recvmsg.Msg)
		}
	case cerr := <-rctx.Done():
		t.Fatalf("test message timed out: %v", cerr)
	}
}

//在两个直接连接的对等端之间发送非对称加密消息
func TestSendAsym(t *testing.T) {
	t.Run("32", testSendAsym)
	t.Run("8", testSendAsym)
	t.Run("0", testSendAsym)
}

func testSendAsym(t *testing.T) {

//地址提示大小
	var addrsize int64
	var err error
	paramstring := strings.Split(t.Name(), "/")
	addrsize, _ = strconv.ParseInt(paramstring[1], 10, 0)
	log.Info("asym send test", "addrsize", addrsize)

	clients, err := setupNetwork(2, false)
	if err != nil {
		t.Fatal(err)
	}

	var topic string
	err = clients[0].Call(&topic, "pss_stringToTopic", "foo:42")
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 250)

	var loaddrhex string
	err = clients[0].Call(&loaddrhex, "pss_baseAddr")
	if err != nil {
		t.Fatalf("rpc get node 1 baseaddr fail: %v", err)
	}
	loaddrhex = loaddrhex[:2+(addrsize*2)]
	var roaddrhex string
	err = clients[1].Call(&roaddrhex, "pss_baseAddr")
	if err != nil {
		t.Fatalf("rpc get node 2 baseaddr fail: %v", err)
	}
	roaddrhex = roaddrhex[:2+(addrsize*2)]

//从PSS实例检索公钥
//互惠设置此公钥
	var lpubkey string
	err = clients[0].Call(&lpubkey, "pss_getPublicKey")
	if err != nil {
		t.Fatalf("rpc get node 1 pubkey fail: %v", err)
	}
	var rpubkey string
	err = clients[1].Call(&rpubkey, "pss_getPublicKey")
	if err != nil {
		t.Fatalf("rpc get node 2 pubkey fail: %v", err)
	}

time.Sleep(time.Millisecond * 500) //替换为配置单元正常代码

	lmsgC := make(chan APIMsg)
	lctx, lcancel := context.WithTimeout(context.Background(), time.Second*10)
	defer lcancel()
	lsub, err := clients[0].Subscribe(lctx, "pss", lmsgC, "receive", topic, false, false)
	log.Trace("lsub", "id", lsub)
	defer lsub.Unsubscribe()
	rmsgC := make(chan APIMsg)
	rctx, rcancel := context.WithTimeout(context.Background(), time.Second*10)
	defer rcancel()
	rsub, err := clients[1].Subscribe(rctx, "pss", rmsgC, "receive", topic, false, false)
	log.Trace("rsub", "id", rsub)
	defer rsub.Unsubscribe()

//存储对等公钥
	err = clients[0].Call(nil, "pss_setPeerPublicKey", rpubkey, topic, roaddrhex)
	if err != nil {
		t.Fatal(err)
	}
	err = clients[1].Call(nil, "pss_setPeerPublicKey", lpubkey, topic, loaddrhex)
	if err != nil {
		t.Fatal(err)
	}

//发送并验证传递
	rmsg := []byte("xyzzy")
	err = clients[0].Call(nil, "pss_sendAsym", rpubkey, topic, hexutil.Encode(rmsg))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case recvmsg := <-rmsgC:
		if !bytes.Equal(recvmsg.Msg, rmsg) {
			t.Fatalf("node 2 received payload mismatch: expected %v, got %v", rmsg, recvmsg.Msg)
		}
	case cerr := <-rctx.Done():
		t.Fatalf("test message timed out: %v", cerr)
	}
	lmsg := []byte("plugh")
	err = clients[1].Call(nil, "pss_sendAsym", lpubkey, topic, hexutil.Encode(lmsg))
	if err != nil {
		t.Fatal(err)
	}
	select {
	case recvmsg := <-lmsgC:
		if !bytes.Equal(recvmsg.Msg, lmsg) {
			t.Fatalf("node 1 received payload mismatch: expected %v, got %v", lmsg, recvmsg.Msg)
		}
	case cerr := <-lctx.Done():
		t.Fatalf("test message timed out: %v", cerr)
	}
}

type Job struct {
	Msg      []byte
	SendNode enode.ID
	RecvNode enode.ID
}

func worker(id int, jobs <-chan Job, rpcs map[enode.ID]*rpc.Client, pubkeys map[enode.ID]string, topic string) {
	for j := range jobs {
		rpcs[j.SendNode].Call(nil, "pss_sendAsym", pubkeys[j.RecvNode], topic, hexutil.Encode(j.Msg))
	}
}

func TestNetwork(t *testing.T) {
	t.Run("16/1000/4/sim", testNetwork)
}

//运行名称中的参数：
//节点/msgs/addrbytes/adaptertype
//如果adaptertype为exec，则使用execadapter，否则使用simadapter
func TestNetwork2000(t *testing.T) {
//Enabl（）

	if !*longrunning {
		t.Skip("run with --longrunning flag to run extensive network tests")
	}
	t.Run("3/2000/4/sim", testNetwork)
	t.Run("4/2000/4/sim", testNetwork)
	t.Run("8/2000/4/sim", testNetwork)
	t.Run("16/2000/4/sim", testNetwork)
}

func TestNetwork5000(t *testing.T) {
//Enabl（）

	if !*longrunning {
		t.Skip("run with --longrunning flag to run extensive network tests")
	}
	t.Run("3/5000/4/sim", testNetwork)
	t.Run("4/5000/4/sim", testNetwork)
	t.Run("8/5000/4/sim", testNetwork)
	t.Run("16/5000/4/sim", testNetwork)
}

func TestNetwork10000(t *testing.T) {
//Enabl（）

	if !*longrunning {
		t.Skip("run with --longrunning flag to run extensive network tests")
	}
	t.Run("3/10000/4/sim", testNetwork)
	t.Run("4/10000/4/sim", testNetwork)
	t.Run("8/10000/4/sim", testNetwork)
}

func testNetwork(t *testing.T) {
	paramstring := strings.Split(t.Name(), "/")
	nodecount, _ := strconv.ParseInt(paramstring[1], 10, 0)
	msgcount, _ := strconv.ParseInt(paramstring[2], 10, 0)
	addrsize, _ := strconv.ParseInt(paramstring[3], 10, 0)
	adapter := paramstring[4]

	log.Info("network test", "nodecount", nodecount, "msgcount", msgcount, "addrhintsize", addrsize)

	nodes := make([]enode.ID, nodecount)
	bzzaddrs := make(map[enode.ID]string, nodecount)
	rpcs := make(map[enode.ID]*rpc.Client, nodecount)
	pubkeys := make(map[enode.ID]string, nodecount)

	sentmsgs := make([][]byte, msgcount)
	recvmsgs := make([]bool, msgcount)
	nodemsgcount := make(map[enode.ID]int, nodecount)

	trigger := make(chan enode.ID)

	var a adapters.NodeAdapter
	if adapter == "exec" {
		dirname, err := ioutil.TempDir(".", "")
		if err != nil {
			t.Fatal(err)
		}
		a = adapters.NewExecAdapter(dirname)
	} else if adapter == "tcp" {
		a = adapters.NewTCPAdapter(newServices(false))
	} else if adapter == "sim" {
		a = adapters.NewSimAdapter(newServices(false))
	}
	net := simulations.NewNetwork(a, &simulations.NetworkConfig{
		ID: "0",
	})
	defer net.Shutdown()

	f, err := os.Open(fmt.Sprintf("testdata/snapshot_%d.json", nodecount))
	if err != nil {
		t.Fatal(err)
	}
	jsonbyte, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	var snap simulations.Snapshot
	err = json.Unmarshal(jsonbyte, &snap)
	if err != nil {
		t.Fatal(err)
	}
	err = net.Load(&snap)
	if err != nil {
//TODO:将p2p仿真框架修复为加载32个节点时不会崩溃
//致死性（Err）
	}

	time.Sleep(1 * time.Second)

	triggerChecks := func(trigger chan enode.ID, id enode.ID, rpcclient *rpc.Client, topic string) error {
		msgC := make(chan APIMsg)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		sub, err := rpcclient.Subscribe(ctx, "pss", msgC, "receive", topic, false, false)
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			defer sub.Unsubscribe()
			for {
				select {
				case recvmsg := <-msgC:
					idx, _ := binary.Uvarint(recvmsg.Msg)
					if !recvmsgs[idx] {
						log.Debug("msg recv", "idx", idx, "id", id)
						recvmsgs[idx] = true
						trigger <- id
					}
				case <-sub.Err():
					return
				}
			}
		}()
		return nil
	}

	var topic string
	for i, nod := range net.GetNodes() {
		nodes[i] = nod.ID()
		rpcs[nodes[i]], err = nod.Client()
		if err != nil {
			t.Fatal(err)
		}
		if topic == "" {
			err = rpcs[nodes[i]].Call(&topic, "pss_stringToTopic", "foo:42")
			if err != nil {
				t.Fatal(err)
			}
		}
		var pubkey string
		err = rpcs[nodes[i]].Call(&pubkey, "pss_getPublicKey")
		if err != nil {
			t.Fatal(err)
		}
		pubkeys[nod.ID()] = pubkey
		var addrhex string
		err = rpcs[nodes[i]].Call(&addrhex, "pss_baseAddr")
		if err != nil {
			t.Fatal(err)
		}
		bzzaddrs[nodes[i]] = addrhex
		err = triggerChecks(trigger, nodes[i], rpcs[nodes[i]], topic)
		if err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(1 * time.Second)

//安装工人
	jobs := make(chan Job, 10)
	for w := 1; w <= 10; w++ {
		go worker(w, jobs, rpcs, pubkeys, topic)
	}

	time.Sleep(1 * time.Second)

	for i := 0; i < int(msgcount); i++ {
		sendnodeidx := rand.Intn(int(nodecount))
		recvnodeidx := rand.Intn(int(nodecount - 1))
		if recvnodeidx >= sendnodeidx {
			recvnodeidx++
		}
		nodemsgcount[nodes[recvnodeidx]]++
		sentmsgs[i] = make([]byte, 8)
		c := binary.PutUvarint(sentmsgs[i], uint64(i))
		if c == 0 {
			t.Fatal("0 byte message")
		}
		if err != nil {
			t.Fatal(err)
		}
		err = rpcs[nodes[sendnodeidx]].Call(nil, "pss_setPeerPublicKey", pubkeys[nodes[recvnodeidx]], topic, bzzaddrs[nodes[recvnodeidx]])
		if err != nil {
			t.Fatal(err)
		}

		jobs <- Job{
			Msg:      sentmsgs[i],
			SendNode: nodes[sendnodeidx],
			RecvNode: nodes[recvnodeidx],
		}
	}

	finalmsgcount := 0
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
outer:
	for i := 0; i < int(msgcount); i++ {
		select {
		case id := <-trigger:
			nodemsgcount[id]--
			finalmsgcount++
		case <-ctx.Done():
			log.Warn("timeout")
			break outer
		}
	}

	for i, msg := range recvmsgs {
		if !msg {
			log.Debug("missing message", "idx", i)
		}
	}
	t.Logf("%d of %d messages received", finalmsgcount, msgcount)

	if finalmsgcount != int(msgcount) {
		t.Fatalf("%d messages were not received", int(msgcount)-finalmsgcount)
	}

}

//在A->B->C->A的网络中检查
//A没有收到两次发送的消息
func TestDeduplication(t *testing.T) {
	var err error

	clients, err := setupNetwork(3, false)
	if err != nil {
		t.Fatal(err)
	}

	var addrsize = 32
	var loaddrhex string
	err = clients[0].Call(&loaddrhex, "pss_baseAddr")
	if err != nil {
		t.Fatalf("rpc get node 1 baseaddr fail: %v", err)
	}
	loaddrhex = loaddrhex[:2+(addrsize*2)]
	var roaddrhex string
	err = clients[1].Call(&roaddrhex, "pss_baseAddr")
	if err != nil {
		t.Fatalf("rpc get node 2 baseaddr fail: %v", err)
	}
	roaddrhex = roaddrhex[:2+(addrsize*2)]
	var xoaddrhex string
	err = clients[2].Call(&xoaddrhex, "pss_baseAddr")
	if err != nil {
		t.Fatalf("rpc get node 3 baseaddr fail: %v", err)
	}
	xoaddrhex = xoaddrhex[:2+(addrsize*2)]

	log.Info("peer", "l", loaddrhex, "r", roaddrhex, "x", xoaddrhex)

	var topic string
	err = clients[0].Call(&topic, "pss_stringToTopic", "foo:42")
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 250)

//从PSS实例检索公钥
//互惠设置此公钥
	var rpubkey string
	err = clients[1].Call(&rpubkey, "pss_getPublicKey")
	if err != nil {
		t.Fatalf("rpc get receivenode pubkey fail: %v", err)
	}

time.Sleep(time.Millisecond * 500) //替换为配置单元正常代码

	rmsgC := make(chan APIMsg)
	rctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()
	rsub, err := clients[1].Subscribe(rctx, "pss", rmsgC, "receive", topic, false, false)
	log.Trace("rsub", "id", rsub)
	defer rsub.Unsubscribe()

//存储收件人的公钥
//零长度地址表示转发给所有人
//我们只有两个对等机，它们将在proxbin中，并且都将接收
	err = clients[0].Call(nil, "pss_setPeerPublicKey", rpubkey, topic, "0x")
	if err != nil {
		t.Fatal(err)
	}

//发送并验证传递
	rmsg := []byte("xyzzy")
	err = clients[0].Call(nil, "pss_sendAsym", rpubkey, topic, hexutil.Encode(rmsg))
	if err != nil {
		t.Fatal(err)
	}

	var receivedok bool
OUTER:
	for {
		select {
		case <-rmsgC:
			if receivedok {
				t.Fatalf("duplicate message received")
			}
			receivedok = true
		case <-rctx.Done():
			break OUTER
		}
	}
	if !receivedok {
		t.Fatalf("message did not arrive")
	}
}

//具有不同消息大小的对称发送性能
func BenchmarkSymkeySend(b *testing.B) {
	b.Run(fmt.Sprintf("%d", 256), benchmarkSymKeySend)
	b.Run(fmt.Sprintf("%d", 1024), benchmarkSymKeySend)
	b.Run(fmt.Sprintf("%d", 1024*1024), benchmarkSymKeySend)
	b.Run(fmt.Sprintf("%d", 1024*1024*10), benchmarkSymKeySend)
	b.Run(fmt.Sprintf("%d", 1024*1024*100), benchmarkSymKeySend)
}

func benchmarkSymKeySend(b *testing.B) {
	msgsizestring := strings.Split(b.Name(), "/")
	if len(msgsizestring) != 2 {
		b.Fatalf("benchmark called without msgsize param")
	}
	msgsize, err := strconv.ParseInt(msgsizestring[1], 10, 0)
	if err != nil {
		b.Fatalf("benchmark called with invalid msgsize param '%s': %v", msgsizestring[1], err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	keys, err := wapi.NewKeyPair(ctx)
	privkey, err := w.GetPrivateKey(keys)
	ps := newTestPss(privkey, nil, nil)
	msg := make([]byte, msgsize)
	rand.Read(msg)
	topic := BytesToTopic([]byte("foo"))
	to := make(PssAddress, 32)
	copy(to[:], network.RandomAddr().Over())
	symkeyid, err := ps.GenerateSymmetricKey(topic, to, true)
	if err != nil {
		b.Fatalf("could not generate symkey: %v", err)
	}
	symkey, err := ps.w.GetSymKey(symkeyid)
	if err != nil {
		b.Fatalf("could not retrieve symkey: %v", err)
	}
	ps.SetSymmetricKey(symkey, topic, to, false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ps.SendSym(symkeyid, topic, msg)
	}
}

//具有不同消息大小的非对称发送性能
func BenchmarkAsymkeySend(b *testing.B) {
	b.Run(fmt.Sprintf("%d", 256), benchmarkAsymKeySend)
	b.Run(fmt.Sprintf("%d", 1024), benchmarkAsymKeySend)
	b.Run(fmt.Sprintf("%d", 1024*1024), benchmarkAsymKeySend)
	b.Run(fmt.Sprintf("%d", 1024*1024*10), benchmarkAsymKeySend)
	b.Run(fmt.Sprintf("%d", 1024*1024*100), benchmarkAsymKeySend)
}

func benchmarkAsymKeySend(b *testing.B) {
	msgsizestring := strings.Split(b.Name(), "/")
	if len(msgsizestring) != 2 {
		b.Fatalf("benchmark called without msgsize param")
	}
	msgsize, err := strconv.ParseInt(msgsizestring[1], 10, 0)
	if err != nil {
		b.Fatalf("benchmark called with invalid msgsize param '%s': %v", msgsizestring[1], err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	keys, err := wapi.NewKeyPair(ctx)
	privkey, err := w.GetPrivateKey(keys)
	ps := newTestPss(privkey, nil, nil)
	msg := make([]byte, msgsize)
	rand.Read(msg)
	topic := BytesToTopic([]byte("foo"))
	to := make(PssAddress, 32)
	copy(to[:], network.RandomAddr().Over())
	ps.SetPeerPublicKey(&privkey.PublicKey, topic, to)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ps.SendAsym(common.ToHex(crypto.FromECDSAPub(&privkey.PublicKey)), topic, msg)
	}
}
func BenchmarkSymkeyBruteforceChangeaddr(b *testing.B) {
	for i := 100; i < 100000; i = i * 10 {
		for j := 32; j < 10000; j = j * 8 {
			b.Run(fmt.Sprintf("%d/%d", i, j), benchmarkSymkeyBruteforceChangeaddr)
		}
//b.run（fmt.sprintf（“%d”，i），BenchmarkSymkeyBruteforceChangeAddr）
	}
}

//使用symkey缓存解密性能，最坏情况下
//（解密密钥始终位于缓存中）
func benchmarkSymkeyBruteforceChangeaddr(b *testing.B) {
	keycountstring := strings.Split(b.Name(), "/")
	cachesize := int64(0)
	var ps *Pss
	if len(keycountstring) < 2 {
		b.Fatalf("benchmark called without count param")
	}
	keycount, err := strconv.ParseInt(keycountstring[1], 10, 0)
	if err != nil {
		b.Fatalf("benchmark called with invalid count param '%s': %v", keycountstring[1], err)
	}
	if len(keycountstring) == 3 {
		cachesize, err = strconv.ParseInt(keycountstring[2], 10, 0)
		if err != nil {
			b.Fatalf("benchmark called with invalid cachesize '%s': %v", keycountstring[2], err)
		}
	}
	pssmsgs := make([]*PssMsg, 0, keycount)
	var keyid string
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	keys, err := wapi.NewKeyPair(ctx)
	privkey, err := w.GetPrivateKey(keys)
	if cachesize > 0 {
		ps = newTestPss(privkey, nil, &PssParams{SymKeyCacheCapacity: int(cachesize)})
	} else {
		ps = newTestPss(privkey, nil, nil)
	}
	topic := BytesToTopic([]byte("foo"))
	for i := 0; i < int(keycount); i++ {
		to := make(PssAddress, 32)
		copy(to[:], network.RandomAddr().Over())
		keyid, err = ps.GenerateSymmetricKey(topic, to, true)
		if err != nil {
			b.Fatalf("cant generate symkey #%d: %v", i, err)
		}
		symkey, err := ps.w.GetSymKey(keyid)
		if err != nil {
			b.Fatalf("could not retrieve symkey %s: %v", keyid, err)
		}
		wparams := &whisper.MessageParams{
			TTL:      defaultWhisperTTL,
			KeySym:   symkey,
			Topic:    whisper.TopicType(topic),
			WorkTime: defaultWhisperWorkTime,
			PoW:      defaultWhisperPoW,
			Payload:  []byte("xyzzy"),
			Padding:  []byte("1234567890abcdef"),
		}
		woutmsg, err := whisper.NewSentMessage(wparams)
		if err != nil {
			b.Fatalf("could not create whisper message: %v", err)
		}
		env, err := woutmsg.Wrap(wparams)
		if err != nil {
			b.Fatalf("could not generate whisper envelope: %v", err)
		}
		ps.Register(&topic, &handler{
			f: noopHandlerFunc,
		})
		pssmsgs = append(pssmsgs, &PssMsg{
			To:      to,
			Payload: env,
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ps.process(pssmsgs[len(pssmsgs)-(i%len(pssmsgs))-1], false, false); err != nil {
			b.Fatalf("pss processing failed: %v", err)
		}
	}
}

func BenchmarkSymkeyBruteforceSameaddr(b *testing.B) {
	for i := 100; i < 100000; i = i * 10 {
		for j := 32; j < 10000; j = j * 8 {
			b.Run(fmt.Sprintf("%d/%d", i, j), benchmarkSymkeyBruteforceSameaddr)
		}
	}
}

//使用symkey缓存解密性能，最佳情况
//（解密密钥总是在缓存中的第一个）
func benchmarkSymkeyBruteforceSameaddr(b *testing.B) {
	var keyid string
	var ps *Pss
	cachesize := int64(0)
	keycountstring := strings.Split(b.Name(), "/")
	if len(keycountstring) < 2 {
		b.Fatalf("benchmark called without count param")
	}
	keycount, err := strconv.ParseInt(keycountstring[1], 10, 0)
	if err != nil {
		b.Fatalf("benchmark called with invalid count param '%s': %v", keycountstring[1], err)
	}
	if len(keycountstring) == 3 {
		cachesize, err = strconv.ParseInt(keycountstring[2], 10, 0)
		if err != nil {
			b.Fatalf("benchmark called with invalid cachesize '%s': %v", keycountstring[2], err)
		}
	}
	addr := make([]PssAddress, keycount)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	keys, err := wapi.NewKeyPair(ctx)
	privkey, err := w.GetPrivateKey(keys)
	if cachesize > 0 {
		ps = newTestPss(privkey, nil, &PssParams{SymKeyCacheCapacity: int(cachesize)})
	} else {
		ps = newTestPss(privkey, nil, nil)
	}
	topic := BytesToTopic([]byte("foo"))
	for i := 0; i < int(keycount); i++ {
		copy(addr[i], network.RandomAddr().Over())
		keyid, err = ps.GenerateSymmetricKey(topic, addr[i], true)
		if err != nil {
			b.Fatalf("cant generate symkey #%d: %v", i, err)
		}

	}
	symkey, err := ps.w.GetSymKey(keyid)
	if err != nil {
		b.Fatalf("could not retrieve symkey %s: %v", keyid, err)
	}
	wparams := &whisper.MessageParams{
		TTL:      defaultWhisperTTL,
		KeySym:   symkey,
		Topic:    whisper.TopicType(topic),
		WorkTime: defaultWhisperWorkTime,
		PoW:      defaultWhisperPoW,
		Payload:  []byte("xyzzy"),
		Padding:  []byte("1234567890abcdef"),
	}
	woutmsg, err := whisper.NewSentMessage(wparams)
	if err != nil {
		b.Fatalf("could not create whisper message: %v", err)
	}
	env, err := woutmsg.Wrap(wparams)
	if err != nil {
		b.Fatalf("could not generate whisper envelope: %v", err)
	}
	ps.Register(&topic, &handler{
		f: noopHandlerFunc,
	})
	pssmsg := &PssMsg{
		To:      addr[len(addr)-1][:],
		Payload: env,
	}
	for i := 0; i < b.N; i++ {
		if err := ps.process(pssmsg, false, false); err != nil {
			b.Fatalf("pss processing failed: %v", err)
		}
	}
}

//使用bzz/discovery和pss服务设置模拟网络。
//连接圆中的节点
//如果设置了allowraw，则启用了省略内置PSS加密（请参阅PSSPARAMS）
func setupNetwork(numnodes int, allowRaw bool) (clients []*rpc.Client, err error) {
	nodes := make([]*simulations.Node, numnodes)
	clients = make([]*rpc.Client, numnodes)
	if numnodes < 2 {
		return nil, fmt.Errorf("Minimum two nodes in network")
	}
	adapter := adapters.NewSimAdapter(newServices(allowRaw))
	net := simulations.NewNetwork(adapter, &simulations.NetworkConfig{
		ID:             "0",
		DefaultService: "bzz",
	})
	for i := 0; i < numnodes; i++ {
		nodeconf := adapters.RandomNodeConfig()
		nodeconf.Services = []string{"bzz", pssProtocolName}
		nodes[i], err = net.NewNodeWithConfig(nodeconf)
		if err != nil {
			return nil, fmt.Errorf("error creating node 1: %v", err)
		}
		err = net.Start(nodes[i].ID())
		if err != nil {
			return nil, fmt.Errorf("error starting node 1: %v", err)
		}
		if i > 0 {
			err = net.Connect(nodes[i].ID(), nodes[i-1].ID())
			if err != nil {
				return nil, fmt.Errorf("error connecting nodes: %v", err)
			}
		}
		clients[i], err = nodes[i].Client()
		if err != nil {
			return nil, fmt.Errorf("create node 1 rpc client fail: %v", err)
		}
	}
	if numnodes > 2 {
		err = net.Connect(nodes[0].ID(), nodes[len(nodes)-1].ID())
		if err != nil {
			return nil, fmt.Errorf("error connecting first and last nodes")
		}
	}
	return clients, nil
}

func newServices(allowRaw bool) adapters.Services {
	stateStore := state.NewInmemoryStore()
	kademlias := make(map[enode.ID]*network.Kademlia)
	kademlia := func(id enode.ID) *network.Kademlia {
		if k, ok := kademlias[id]; ok {
			return k
		}
		params := network.NewKadParams()
		params.NeighbourhoodSize = 2
		params.MaxBinSize = 3
		params.MinBinSize = 1
		params.MaxRetries = 1000
		params.RetryExponent = 2
		params.RetryInterval = 1000000
		kademlias[id] = network.NewKademlia(id[:], params)
		return kademlias[id]
	}
	return adapters.Services{
		pssProtocolName: func(ctx *adapters.ServiceContext) (node.Service, error) {
//execadapter不执行init（）。
			initTest()

			ctxlocal, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			keys, err := wapi.NewKeyPair(ctxlocal)
			privkey, err := w.GetPrivateKey(keys)
			pssp := NewPssParams().WithPrivateKey(privkey)
			pssp.AllowRaw = allowRaw
			pskad := kademlia(ctx.Config.ID)
			ps, err := NewPss(pskad, pssp)
			if err != nil {
				return nil, err
			}

			ping := &Ping{
				OutC: make(chan bool),
				Pong: true,
			}
			p2pp := NewPingProtocol(ping)
			pp, err := RegisterProtocol(ps, &PingTopic, PingProtocol, p2pp, &ProtocolParams{Asymmetric: true})
			if err != nil {
				return nil, err
			}
			if useHandshake {
				SetHandshakeController(ps, NewHandshakeParams())
			}
			ps.Register(&PingTopic, &handler{
				f: pp.Handle,
				caps: &handlerCaps{
					raw: true,
				},
			})
			ps.addAPI(rpc.API{
				Namespace: "psstest",
				Version:   "0.3",
				Service:   NewAPITest(ps),
				Public:    false,
			})
			if err != nil {
				log.Error("Couldnt register pss protocol", "err", err)
				os.Exit(1)
			}
			pssprotocols[ctx.Config.ID.String()] = &protoCtrl{
				C:        ping.OutC,
				protocol: pp,
				run:      p2pp.Run,
			}
			return ps, nil
		},
		"bzz": func(ctx *adapters.ServiceContext) (node.Service, error) {
			addr := network.NewAddr(ctx.Config.Node())
			hp := network.NewHiveParams()
			hp.Discovery = false
			config := &network.BzzConfig{
				OverlayAddr:  addr.Over(),
				UnderlayAddr: addr.Under(),
				HiveParams:   hp,
			}
			return network.NewBzz(config, kademlia(ctx.Config.ID), stateStore, nil, nil), nil
		},
	}
}

func newTestPss(privkey *ecdsa.PrivateKey, kad *network.Kademlia, ppextra *PssParams) *Pss {
	nid := enode.PubkeyToIDV4(&privkey.PublicKey)
//如果Kademlia未传递给我们，则设置路由
	if kad == nil {
		kp := network.NewKadParams()
		kp.NeighbourhoodSize = 3
		kad = network.NewKademlia(nid[:], kp)
	}

//创建PSS
	pp := NewPssParams().WithPrivateKey(privkey)
	if ppextra != nil {
		pp.SymKeyCacheCapacity = ppextra.SymKeyCacheCapacity
	}
	ps, err := NewPss(kad, pp)
	if err != nil {
		return nil
	}
	ps.Start(nil)

	return ps
}

//API要求测试/开发使用
type APITest struct {
	*Pss
}

func NewAPITest(ps *Pss) *APITest {
	return &APITest{Pss: ps}
}

func (apitest *APITest) SetSymKeys(pubkeyid string, recvsymkey []byte, sendsymkey []byte, limit uint16, topic Topic, to hexutil.Bytes) ([2]string, error) {

	recvsymkeyid, err := apitest.SetSymmetricKey(recvsymkey, topic, PssAddress(to), true)
	if err != nil {
		return [2]string{}, err
	}
	sendsymkeyid, err := apitest.SetSymmetricKey(sendsymkey, topic, PssAddress(to), false)
	if err != nil {
		return [2]string{}, err
	}
	return [2]string{recvsymkeyid, sendsymkeyid}, nil
}

func (apitest *APITest) Clean() (int, error) {
	return apitest.Pss.cleanKeys(), nil
}

//EnableMetrics正在启动InfluxDB Reporter，以便在本地运行测试时收集统计信息
func enableMetrics() {
	metrics.Enabled = true
go influxdb.InfluxDBWithTags(metrics.DefaultRegistry, 1*time.Second, "http://localhost:8086，“metrics”，“admin”，“admin”，“swarm.”，map[string]string_
		"host": "test",
	})
}
