
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

//+建立FO

package pss

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/swarm/log"
)

//两个直接连接的对等端之间的非对称密钥交换
//完整地址、部分地址（8字节）和空地址
func TestHandshake(t *testing.T) {
	t.Skip("handshakes are not adapted to current pss core code")
	t.Run("32", testHandshake)
	t.Run("8", testHandshake)
	t.Run("0", testHandshake)
}

func testHandshake(t *testing.T) {

//我们要用多少地址
	useHandshake = true
	var addrsize int64
	var err error
	addrsizestring := strings.Split(t.Name(), "/")
	addrsize, _ = strconv.ParseInt(addrsizestring[1], 10, 0)

//设置两个直接连接的节点
//（我们不在这里测试PSS路由）
	clients, err := setupNetwork(2)
	if err != nil {
		t.Fatal(err)
	}

	var topic string
	err = clients[0].Call(&topic, "pss_stringToTopic", "foo:42")
	if err != nil {
		t.Fatal(err)
	}

	var loaddr string
	err = clients[0].Call(&loaddr, "pss_baseAddr")
	if err != nil {
		t.Fatalf("rpc get node 1 baseaddr fail: %v", err)
	}
//“0x”=2字节+addrsize地址字节，十六进制为2x长度
	loaddr = loaddr[:2+(addrsize*2)]
	var roaddr string
	err = clients[1].Call(&roaddr, "pss_baseAddr")
	if err != nil {
		t.Fatalf("rpc get node 2 baseaddr fail: %v", err)
	}
	roaddr = roaddr[:2+(addrsize*2)]
	log.Debug("addresses", "left", loaddr, "right", roaddr)

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

time.Sleep(time.Millisecond * 1000) //替换为配置单元正常代码

//为每个节点提供其对等方的公钥
	err = clients[0].Call(nil, "pss_setPeerPublicKey", rpubkey, topic, roaddr)
	if err != nil {
		t.Fatal(err)
	}
	err = clients[1].Call(nil, "pss_setPeerPublicKey", lpubkey, topic, loaddr)
	if err != nil {
		t.Fatal(err)
	}

//握手
//在此之后，每侧将具有默认symkeybuffercapacity symkey，分别用于输入和输出消息：
//L->REQUEST 4键->R
//L<-发送4个键，请求4个键<-R
//L->SEND 4键->R
//调用将用L发送到R所需的符号键填充数组。
	err = clients[0].Call(nil, "pss_addHandshake", topic)
	if err != nil {
		t.Fatal(err)
	}
	err = clients[1].Call(nil, "pss_addHandshake", topic)
	if err != nil {
		t.Fatal(err)
	}

	var lhsendsymkeyids []string
	err = clients[0].Call(&lhsendsymkeyids, "pss_handshake", rpubkey, topic, true, true)
	if err != nil {
		t.Fatal(err)
	}

//确保R节点获取其键
	time.Sleep(time.Second)

//检查我们是否存储了6个传出密钥，它们与从R收到的密钥匹配
	var lsendsymkeyids []string
	err = clients[0].Call(&lsendsymkeyids, "pss_getHandshakeKeys", rpubkey, topic, false, true)
	if err != nil {
		t.Fatal(err)
	}
	m := 0
	for _, hid := range lhsendsymkeyids {
		for _, lid := range lsendsymkeyids {
			if lid == hid {
				m++
			}
		}
	}
	if m != defaultSymKeyCapacity {
		t.Fatalf("buffer size mismatch, expected %d, have %d: %v", defaultSymKeyCapacity, m, lsendsymkeyids)
	}

//检查l-node和r-node上的输入和输出键是否匹配，是否在相反的类别中（l recv=r send，l send=r recv）
	var rsendsymkeyids []string
	err = clients[1].Call(&rsendsymkeyids, "pss_getHandshakeKeys", lpubkey, topic, false, true)
	if err != nil {
		t.Fatal(err)
	}
	var lrecvsymkeyids []string
	err = clients[0].Call(&lrecvsymkeyids, "pss_getHandshakeKeys", rpubkey, topic, true, false)
	if err != nil {
		t.Fatal(err)
	}
	var rrecvsymkeyids []string
	err = clients[1].Call(&rrecvsymkeyids, "pss_getHandshakeKeys", lpubkey, topic, true, false)
	if err != nil {
		t.Fatal(err)
	}

//从两侧获取字节形式的传出符号键
	var lsendsymkeys []string
	for _, id := range lsendsymkeyids {
		var key string
		err = clients[0].Call(&key, "pss_getSymmetricKey", id)
		if err != nil {
			t.Fatal(err)
		}
		lsendsymkeys = append(lsendsymkeys, key)
	}
	var rsendsymkeys []string
	for _, id := range rsendsymkeyids {
		var key string
		err = clients[1].Call(&key, "pss_getSymmetricKey", id)
		if err != nil {
			t.Fatal(err)
		}
		rsendsymkeys = append(rsendsymkeys, key)
	}

//从两侧获取字节形式的传入符号键并进行比较
	var lrecvsymkeys []string
	for _, id := range lrecvsymkeyids {
		var key string
		err = clients[0].Call(&key, "pss_getSymmetricKey", id)
		if err != nil {
			t.Fatal(err)
		}
		match := false
		for _, otherkey := range rsendsymkeys {
			if otherkey == key {
				match = true
			}
		}
		if !match {
			t.Fatalf("no match right send for left recv key %s", id)
		}
		lrecvsymkeys = append(lrecvsymkeys, key)
	}
	var rrecvsymkeys []string
	for _, id := range rrecvsymkeyids {
		var key string
		err = clients[1].Call(&key, "pss_getSymmetricKey", id)
		if err != nil {
			t.Fatal(err)
		}
		match := false
		for _, otherkey := range lsendsymkeys {
			if otherkey == key {
				match = true
			}
		}
		if !match {
			t.Fatalf("no match left send for right recv key %s", id)
		}
		rrecvsymkeys = append(rrecvsymkeys, key)
	}

//发送新的握手请求，不应发送密钥
	err = clients[0].Call(nil, "pss_handshake", rpubkey, topic, false)
	if err == nil {
		t.Fatal("expected full symkey buffer error")
	}

//使一个密钥过期，发送新的握手请求
	err = clients[0].Call(nil, "pss_releaseHandshakeKey", rpubkey, topic, lsendsymkeyids[0], true)
	if err != nil {
		t.Fatalf("release left send key %s fail: %v", lsendsymkeyids[0], err)
	}

	var newlhsendkeyids []string

//发送新的握手请求，现在应该收到一个密钥
//检查它是否不在上一个右recv key数组中
	err = clients[0].Call(&newlhsendkeyids, "pss_handshake", rpubkey, topic, true, false)
	if err != nil {
		t.Fatalf("handshake send fail: %v", err)
	} else if len(newlhsendkeyids) != defaultSymKeyCapacity {
		t.Fatalf("wrong receive count, expected 1, got %d", len(newlhsendkeyids))
	}

	var newlrecvsymkey string
	err = clients[0].Call(&newlrecvsymkey, "pss_getSymmetricKey", newlhsendkeyids[0])
	if err != nil {
		t.Fatal(err)
	}
	var rmatchsymkeyid *string
	for i, id := range rrecvsymkeyids {
		var key string
		err = clients[1].Call(&key, "pss_getSymmetricKey", id)
		if err != nil {
			t.Fatal(err)
		}
		if newlrecvsymkey == key {
			rmatchsymkeyid = &rrecvsymkeyids[i]
		}
	}
	if rmatchsymkeyid != nil {
		t.Fatalf("right sent old key id %s in second handshake", *rmatchsymkeyid)
	}

//清理PSS核心密钥库。应该清洗之前释放的钥匙
	var cleancount int
	clients[0].Call(&cleancount, "psstest_clean")
	if cleancount > 1 {
		t.Fatalf("pss clean count mismatch; expected 1, got %d", cleancount)
	}
}
