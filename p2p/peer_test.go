
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2014 Go Ethereum作者
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

package p2p

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"testing"
	"time"
)

var discard = Protocol{
	Name:   "discard",
	Length: 1,
	Run: func(p *Peer, rw MsgReadWriter) error {
		for {
			msg, err := rw.ReadMsg()
			if err != nil {
				return err
			}
			fmt.Printf("discarding %d\n", msg.Code)
			if err = msg.Discard(); err != nil {
				return err
			}
		}
	},
}

func testPeer(protos []Protocol) (func(), *conn, *Peer, <-chan error) {
	fd1, fd2 := net.Pipe()
	c1 := &conn{fd: fd1, node: newNode(randomID(), nil), transport: newTestTransport(&newkey().PublicKey, fd1)}
	c2 := &conn{fd: fd2, node: newNode(randomID(), nil), transport: newTestTransport(&newkey().PublicKey, fd2)}
	for _, p := range protos {
		c1.caps = append(c1.caps, p.cap())
		c2.caps = append(c2.caps, p.cap())
	}

	peer := newPeer(c1, protos)
	errc := make(chan error, 1)
	go func() {
		_, err := peer.run()
		errc <- err
	}()

	closer := func() { c2.close(errors.New("close func called")) }
	return closer, c2, peer, errc
}

func TestPeerProtoReadMsg(t *testing.T) {
	proto := Protocol{
		Name:   "a",
		Length: 5,
		Run: func(peer *Peer, rw MsgReadWriter) error {
			if err := ExpectMsg(rw, 2, []uint{1}); err != nil {
				t.Error(err)
			}
			if err := ExpectMsg(rw, 3, []uint{2}); err != nil {
				t.Error(err)
			}
			if err := ExpectMsg(rw, 4, []uint{3}); err != nil {
				t.Error(err)
			}
			return nil
		},
	}

	closer, rw, _, errc := testPeer([]Protocol{proto})
	defer closer()

	Send(rw, baseProtocolLength+2, []uint{1})
	Send(rw, baseProtocolLength+3, []uint{2})
	Send(rw, baseProtocolLength+4, []uint{3})

	select {
	case err := <-errc:
		if err != errProtocolReturned {
			t.Errorf("peer returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Errorf("receive timeout")
	}
}

func TestPeerProtoEncodeMsg(t *testing.T) {
	proto := Protocol{
		Name:   "a",
		Length: 2,
		Run: func(peer *Peer, rw MsgReadWriter) error {
			if err := SendItems(rw, 2); err == nil {
				t.Error("expected error for out-of-range msg code, got nil")
			}
			if err := SendItems(rw, 1, "foo", "bar"); err != nil {
				t.Errorf("write error: %v", err)
			}
			return nil
		},
	}
	closer, rw, _, _ := testPeer([]Protocol{proto})
	defer closer()

	if err := ExpectMsg(rw, 17, []string{"foo", "bar"}); err != nil {
		t.Error(err)
	}
}

func TestPeerPing(t *testing.T) {
	closer, rw, _, _ := testPeer(nil)
	defer closer()
	if err := SendItems(rw, pingMsg); err != nil {
		t.Fatal(err)
	}
	if err := ExpectMsg(rw, pongMsg, nil); err != nil {
		t.Error(err)
	}
}

func TestPeerDisconnect(t *testing.T) {
	closer, rw, _, disc := testPeer(nil)
	defer closer()
	if err := SendItems(rw, discMsg, DiscQuitting); err != nil {
		t.Fatal(err)
	}
	select {
	case reason := <-disc:
		if reason != DiscQuitting {
			t.Errorf("run returned wrong reason: got %v, want %v", reason, DiscQuitting)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("peer did not return")
	}
}

//此测试旨在验证对等端是否能够可靠地处理
//同时发生多个断开原因。
func TestPeerDisconnectRace(t *testing.T) {
	maybe := func() bool { return rand.Intn(1) == 1 }

	for i := 0; i < 1000; i++ {
		protoclose := make(chan error)
		protodisc := make(chan DiscReason)
		closer, rw, p, disc := testPeer([]Protocol{
			{
				Name:   "closereq",
				Run:    func(p *Peer, rw MsgReadWriter) error { return <-protoclose },
				Length: 1,
			},
			{
				Name:   "disconnect",
				Run:    func(p *Peer, rw MsgReadWriter) error { p.Disconnect(<-protodisc); return nil },
				Length: 1,
			},
		})

//模拟传入消息。
		go SendItems(rw, baseProtocolLength+1)
		go SendItems(rw, baseProtocolLength+2)
//关闭网络连接。
		go closer()
//使协议“closereq”返回。
		protoclose <- errors.New("protocol closed")
//使协议“断开”呼叫对等方。断开连接
		protodisc <- DiscAlreadyConnected
//在某些情况下，模拟调用peer.disconnect的其他内容。
		if maybe() {
			go p.Disconnect(DiscInvalidIdentity)
		}
//在某些情况下，模拟远程请求断开连接。
		if maybe() {
			go SendItems(rw, discMsg, DiscQuitting)
		}

		select {
		case <-disc:
		case <-time.After(2 * time.Second):
//peer.run应该很快返回。如果不是同伴
//Goroutines可能陷入了僵局。呼叫恐慌以便
//显示堆栈。
			panic("Peer.run took to long to return.")
		}
	}
}

func TestNewPeer(t *testing.T) {
	name := "nodename"
	caps := []Cap{{"foo", 2}, {"bar", 3}}
	id := randomID()
	p := NewPeer(id, name, caps)
	if p.ID() != id {
		t.Errorf("ID mismatch: got %v, expected %v", p.ID(), id)
	}
	if p.Name() != name {
		t.Errorf("Name mismatch: got %v, expected %v", p.Name(), name)
	}
	if !reflect.DeepEqual(p.Caps(), caps) {
		t.Errorf("Caps mismatch: got %v, expected %v", p.Caps(), caps)
	}

p.Disconnect(DiscAlreadyConnected) //不应该挂
}

func TestMatchProtocols(t *testing.T) {
	tests := []struct {
		Remote []Cap
		Local  []Protocol
		Match  map[string]protoRW
	}{
		{
//无远程功能
			Local: []Protocol{{Name: "a"}},
		},
		{
//没有本地协议
			Remote: []Cap{{Name: "a"}},
		},
		{
//没有相互协议
			Remote: []Cap{{Name: "a"}},
			Local:  []Protocol{{Name: "b"}},
		},
		{
//一些匹配，一些差异
			Remote: []Cap{{Name: "local"}, {Name: "match1"}, {Name: "match2"}},
			Local:  []Protocol{{Name: "match1"}, {Name: "match2"}, {Name: "remote"}},
			Match:  map[string]protoRW{"match1": {Protocol: Protocol{Name: "match1"}}, "match2": {Protocol: Protocol{Name: "match2"}}},
		},
		{
//各种字母顺序
			Remote: []Cap{{Name: "aa"}, {Name: "ab"}, {Name: "bb"}, {Name: "ba"}},
			Local:  []Protocol{{Name: "ba"}, {Name: "bb"}, {Name: "ab"}, {Name: "aa"}},
			Match:  map[string]protoRW{"aa": {Protocol: Protocol{Name: "aa"}}, "ab": {Protocol: Protocol{Name: "ab"}}, "ba": {Protocol: Protocol{Name: "ba"}}, "bb": {Protocol: Protocol{Name: "bb"}}},
		},
		{
//没有相互的版本
			Remote: []Cap{{Version: 1}},
			Local:  []Protocol{{Version: 2}},
		},
		{
//多版本，单通用
			Remote: []Cap{{Version: 1}, {Version: 2}},
			Local:  []Protocol{{Version: 2}, {Version: 3}},
			Match:  map[string]protoRW{"": {Protocol: Protocol{Version: 2}}},
		},
		{
//多版本，多公共
			Remote: []Cap{{Version: 1}, {Version: 2}, {Version: 3}, {Version: 4}},
			Local:  []Protocol{{Version: 2}, {Version: 3}},
			Match:  map[string]protoRW{"": {Protocol: Protocol{Version: 3}}},
		},
		{
//各种版本订单
			Remote: []Cap{{Version: 4}, {Version: 1}, {Version: 3}, {Version: 2}},
			Local:  []Protocol{{Version: 2}, {Version: 3}, {Version: 1}},
			Match:  map[string]protoRW{"": {Protocol: Protocol{Version: 3}}},
		},
		{
//覆盖子协议长度的版本
			Remote: []Cap{{Version: 1}, {Version: 2}, {Version: 3}, {Name: "a"}},
			Local:  []Protocol{{Version: 1, Length: 1}, {Version: 2, Length: 2}, {Version: 3, Length: 3}, {Name: "a"}},
			Match:  map[string]protoRW{"": {Protocol: Protocol{Version: 3}}, "a": {Protocol: Protocol{Name: "a"}, offset: 3}},
		},
	}

	for i, tt := range tests {
		result := matchProtocols(tt.Local, tt.Remote, nil)
		if len(result) != len(tt.Match) {
			t.Errorf("test %d: negotiation mismatch: have %v, want %v", i, len(result), len(tt.Match))
			continue
		}
//确保所有协商的协议都是必要的和正确的
		for name, proto := range result {
			match, ok := tt.Match[name]
			if !ok {
				t.Errorf("test %d, proto '%s': negotiated but shouldn't have", i, name)
				continue
			}
			if proto.Name != match.Name {
				t.Errorf("test %d, proto '%s': name mismatch: have %v, want %v", i, name, proto.Name, match.Name)
			}
			if proto.Version != match.Version {
				t.Errorf("test %d, proto '%s': version mismatch: have %v, want %v", i, name, proto.Version, match.Version)
			}
			if proto.offset-baseProtocolLength != match.offset {
				t.Errorf("test %d, proto '%s': offset mismatch: have %v, want %v", i, name, proto.offset-baseProtocolLength, match.offset)
			}
		}
//确保没有协议错过协商
		for name := range tt.Match {
			if _, ok := result[name]; !ok {
				t.Errorf("test %d, proto '%s': not negotiated, should have", i, name)
				continue
			}
		}
	}
}
