
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

package discv5

import (
	"encoding/hex"
	"errors"
	"io"
	"net"
	"reflect"
	"sync"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

func init() {
	spew.Config.DisableMethods = true
}

//共享测试变量
var (
	testLocal = rpcEndpoint{IP: net.ParseIP("3.3.3.3").To4(), UDP: 5, TCP: 6}
)

//类型udptest struct_
//t*测试.t
//管道*柴油管道
//表*表
//UDP*UDP协议
//发送了[]字节
//本地密钥，远程密钥*ecdsa.privatekey
//远程地址*net.udpaddr
//}
//
//func newudptest（t*测试.t）*udptest
//测试：=&udptest
//T：
//管道：newPipe（），
//localkey:newkey（），
//遥控键：newkey（），
//远程地址：&net.udpaddr ip:net.ip 1、2、3、4，端口：30303，
//}
//test.table，test.udp，=newudp（test.localkey，test.pipe，nil，“”）
//返回测试
//}
//
////处理一个数据包，就像它被发送到传输一样。
//func（test*udptest）packetin（wanterror错误，ptype字节，数据包）错误
//enc，err:=编码包（test.remotekey，ptype，data）
//如果犯错！= nIL{
//返回test.errorf（“数据包（%d）编码错误：%v”，ptype，err）
//}
//test.sent=附加（test.sent，enc）
//如果err=test.udp.handlepacket（test.remoteaddr，enc）；err！=万恐怖
//返回test.errorf（“错误不匹配：得到%q，想要%q”，err，want error）
//}
//返回零
//}
//
////等待传输发送数据包。
////validate应具有func（*udptest，x）类型错误，其中x是数据包类型。
//func（test*udptest）waitpacketout（validate interface）错误
//dgram：=test.pipe.waitpacketout（）。
//P，uuuuerr：=解码包（dgram）
//如果犯错！= nIL{
//返回test.errorf（“发送数据包解码错误：%v”，err）
//}
//fn：=reflect.valueof（验证）
//exptype：=fn.type（）.in（0）
//如果反射。类型（P）！= ExpType {
//return test.errorf（“发送的数据包类型不匹配，得到的数据包类型为%v，想要的数据包类型为%v”，reflect.typeof（p），exptype）
//}
//fn.调用（[]reflect.value reflect.valueof（p））
//返回零
//}
//
//func（test*udptest）errorf（格式字符串，参数…接口）error
//_u，file，line，ok：=runtime.caller（2）//errorf+waitpacketout
//如果OK {
//文件=filepath.base（文件）
//}否则{
//文件=？？？”
//线＝1
//}
//错误：=fmt.errorf（格式，参数…）
//fmt.printf（“\t%s:%d:%v\n”，文件，行，错误）
//测试失败（）
//返回错误
//}
//
//func testudp_packeterrors（t*testing.t）
//测试：=newudptest（t）
//推迟test.table.close（）。
//
//test.packetin（errfired、pingpacket和ping从：testmemote到：testlocallowered，版本：version）
//test.packetin（errUnsolicitedReply、pongPacket和pong replytok:[]byte，expiration:futureexp）
//test.packetin（errunknownnode，findnodepacket，&findnode expiration:futureexp）
//test.packetin（errUnsolicitedReply、neighborsPacket和neighbors expiration:futureexp）
//}
//
//func testudp_findnode（t*testing.t）
//测试：=newudptest（t）
//推迟test.table.close（）。
//
////在表中放入几个节点。他们确切
////分配不太重要，尽管我们需要
////注意不要溢出任何桶。
//targetHash：=crypto.keccak256hash（testTarget[：]）
//节点：=&nodesbyDistance目标：targetHash
//对于i：=0；i<bucketsize；i++
//nodes.push（nodeatDistance（test.table.self.sha，i+2），bucketsize）
//}
//test.table.stuff（nodes.entries）
//
////确保与测试节点有连接，
////否则将不接受findnode。
//test.table.db.updatenode（newnode（
//pubkeyid（&test.remotekey.publickey），。
//测试.remoteaddr.ip，
//uint16（test.remoteaddr.port）
//99，
//）
////检查是否返回最近的邻居。
//test.packetin（nil，findnodepacket，&findnode target:test target，expiration:futureexp）
//应输入：=test.table.closest（targetHash，bucketsize）
//
//waitneighbors：=func（want[]*节点）
//test.waitpacketout（func（p*邻居）
//如果len（p.nodes）！= LEN（WAY）{
//t.errorf（“错误的结果数：得到%d，想要%d”，len（p.nodes），bucketsize）
//}
//对于i：=范围p.节点
//如果p.nodes[i].id！=想要[ i]。ID {
//t.errorf（“结果在%d不匹配：\n得到的：%v\n想要的：%v”，i，p.nodes[i]，需要的是.entries[i]）
//}
//}
//}）
//}
//waitneighbors（应为.entries[：maxneighbors]）
//waitneighbors（应为.entries[maxneighbors:）
//}
//
//func testudp_findnodemultirreply（t*testing.t）
//测试：=newudptest（t）
//推迟test.table.close（）。
//
////将挂起的findnode请求排队
//结果c，errc：=make（chan[]*节点），make（chan错误）
//转到函数（）
//rid：=pubkeyid（&test.remotekey.publickey）
//ns，err：=test.udp.findnode（rid，test.remoteaddr，测试目标）
//如果犯错！=nil和len（ns）==0_
//Err＜Err
//}否则{
//结果<<NS
//}
//}（）
//
////等待发送findnode。
////发送后，传输正在等待答复
//测试.waitpacketout（func（p*findnode）
//如果P目标！=测试目标{
//t.errorf（“错误目标：得到%v，想要%v”，p.target，testtarget）
//}
//}）
//
////以两个数据包的形式发送回复。
//列表：=[]*节点
//必须分析节点（“enode://ba8501c70bcc5c04d8607d3a0ed29aa6179c092cbdda10d5d2684fb33ed01bd94f588ca8f91ac48318087dcb02eaf36773a7a453f0eedd6742af668097b29c@10.0.1.16:30303”？discport=30304”），
//mustParseNode（“enode://81fa361d25f157cd421c60dcc28d8dac5ef6a89476633339c5df30287474520caa09627da18543d9079b5b288698b542d56167aa5c09111e55acdbbdf2ef799@10.0.1.16:30303”），
//must parsenode（“enode://9bffeffd833d53fac8e652415f4973be289e8ba5c6c4cbe70abf817ce8a64cee1b823b66a987f51aaa9fbad6a91b3e6bf0d5a5d1042de8e9ee057b217f8@10.0.1.36:30301？”CdPoT＝17“”，
//mustParseNode（“enode://1b5b4aa662d7cb44a7221bfba67302590b643028197a7d214790f3bac7aa4a32441be9e83c09cf1f6c69d007c634faae3dc1b221793e8446c0b3a09de65960@10.0.1.16:30303”），
//}
//rpclist：=make（[]rpcnode，len（list））。
//对于i：=范围列表
//rpclist[i]=nodetorpc（列表[i]）
//}
//test.packetin（nil，neighborspacket，&neighbors expiration:futureexp，nodes:rpclist[：2]）
//test.packetin（nil，neighborspacket，&neighbors expiration:futureexp，nodes:rpclist[2:]）
//
////检查发送的邻居是否都由findnode返回
//选择{
//案例结果：=<-结果C：
//如果！反映.深度相等（结果，列表）
//t.errorf（“邻居不匹配：\n获得：%v\n想要：%v”，结果，列表）
//}
//大小写错误：=<-errc:
//t.errorf（“findnode错误：%v”，err）
//case<-time.after（5*time.second）：
//t.error（“findnode在5秒内未返回”）
//}
//}
//
//func testudp_成功（t*testing.t）
//测试：=newudptest（t）
//添加：=make（chan*节点，1）
//test.table.nodeadhook=func（n*node）added<-n
//推迟test.table.close（）。
//
////远程端发送一个ping包来启动交换。
//go test.packettin（nil，pingpacket，&ping从：testmremote，到：testloclaunted，版本：version，过期：futureexp）
//
////ping被回复。
//测试.waitpacketout（func（p*pong）
//PingHash：=测试。已发送[0][：MacSize]
//如果！bytes.equal（p.replytok，pinghash）
//t.errorf（“got pong.replytok%x，want%x”，p.replytok，pinghash）
//}
//wantto：=rpcendpoint_
////镜像的udp地址是udp包发送器
//ip:test.remoteaddr.ip，udp:uint16（test.remoteaddr.port），
////镜像TCP端口是来自ping包的端口
//tcp:测试远程.tcp，
//}
//如果！反省。深相等（P.to，Wantto）
//t.errorf（“pong.to%v，want%v”，p.to，want to）
//}
//}）
//
////远程未知，表返回ping。
//test.waitpacketout（func（p*ping）错误
//如果！反射.deepequal（p.from，test.udp.ourendpoint）
//t.errorf（“从%v得到ping.from，想要%v”，p.from，test.udp.ourendpoint）
//}
//wantto：=rpcendpoint_
////镜像的UDP地址是UDP数据包发送器。
//ip:test.remoteaddr.ip，udp:uint16（test.remoteaddr.port），
//TCP：0，
//}
//如果！反省。深相等（P.to，Wantto）
//t.errorf（“得到ping.to%v，想要%v”，p.to，want to）
//}
//返回零
//}）
//测试.包装纸（无、包装纸和包装到期日：FutureExp）
//
////在获取
///P/包。
//选择{
//案例n：=<-添加：
//rid：=pubkeyid（&test.remotekey.publickey）
//如果N.ID！= RID{
//t.errorf（“节点的ID错误：得到了%v，想要%v”，n.id，rid）
//}
//如果！bytes.equal（n.ip，test.remoteaddr.ip）
//t.errorf（“节点的IP错误：得到了%v，想要的是：%v”，n.ip，test.remoteaddr.ip）
//}
//如果int（N.UDP）！=test.remoteaddr.port_
//t.errorf（“节点有错误的udp端口：已获取%v，需要：%v”，n.udp，test.remoteaddr.port）
//}
//如果N.TCP！=测试远程.tcp
//t.errorf（“节点有错误的TCP端口：获取了%v，需要：%v”，n.tcp，testmremote.tcp）
//}
//case<-time.after（2*time.second）：
//t.errorf（“2秒内未添加节点”）
//}
//}

var testPackets = []struct {
	input      string
	wantPacket interface{}
}{
	{
		input: "71dbda3a79554728d4f94411e42ee1f8b0d561c10e1e5f5893367948c6a7d70bb87b235fa28a77070271b6c164a2dce8c7e13a5739b53b5e96f2e5acb0e458a02902f5965d55ecbeb2ebb6cabb8b2b232896a36b737666c55265ad0a68412f250001ea04cb847f000001820cfa8215a8d790000000000000000000000000000000018208ae820d058443b9a355",
		wantPacket: &ping{
			Version:    4,
			From:       rpcEndpoint{net.ParseIP("127.0.0.1").To4(), 3322, 5544},
			To:         rpcEndpoint{net.ParseIP("::1"), 2222, 3333},
			Expiration: 1136239445,
			Rest:       []rlp.RawValue{},
		},
	},
	{
		input: "e9614ccfd9fc3e74360018522d30e1419a143407ffcce748de3e22116b7e8dc92ff74788c0b6663aaa3d67d641936511c8f8d6ad8698b820a7cf9e1be7155e9a241f556658c55428ec0563514365799a4be2be5a685a80971ddcfa80cb422cdd0101ec04cb847f000001820cfa8215a8d790000000000000000000000000000000018208ae820d058443b9a3550102",
		wantPacket: &ping{
			Version:    4,
			From:       rpcEndpoint{net.ParseIP("127.0.0.1").To4(), 3322, 5544},
			To:         rpcEndpoint{net.ParseIP("::1"), 2222, 3333},
			Expiration: 1136239445,
			Rest:       []rlp.RawValue{{0x01}, {0x02}},
		},
	},
	{
		input: "577be4349c4dd26768081f58de4c6f375a7a22f3f7adda654d1428637412c3d7fe917cadc56d4e5e7ffae1dbe3efffb9849feb71b262de37977e7c7a44e677295680e9e38ab26bee2fcbae207fba3ff3d74069a50b902a82c9903ed37cc993c50001f83e82022bd79020010db83c4d001500000000abcdef12820cfa8215a8d79020010db885a308d313198a2e037073488208ae82823a8443b9a355c5010203040531b9019afde696e582a78fa8d95ea13ce3297d4afb8ba6433e4154caa5ac6431af1b80ba76023fa4090c408f6b4bc3701562c031041d4702971d102c9ab7fa5eed4cd6bab8f7af956f7d565ee1917084a95398b6a21eac920fe3dd1345ec0a7ef39367ee69ddf092cbfe5b93e5e568ebc491983c09c76d922dc3",
		wantPacket: &ping{
			Version:    555,
			From:       rpcEndpoint{net.ParseIP("2001:db8:3c4d:15::abcd:ef12"), 3322, 5544},
			To:         rpcEndpoint{net.ParseIP("2001:db8:85a3:8d3:1319:8a2e:370:7348"), 2222, 33338},
			Expiration: 1136239445,
			Rest:       []rlp.RawValue{{0xC5, 0x01, 0x02, 0x03, 0x04, 0x05}},
		},
	},
	{
		input: "09b2428d83348d27cdf7064ad9024f526cebc19e4958f0fdad87c15eb598dd61d08423e0bf66b2069869e1724125f820d851c136684082774f870e614d95a2855d000f05d1648b2d5945470bc187c2d2216fbe870f43ed0909009882e176a46b0102f846d79020010db885a308d313198a2e037073488208ae82823aa0fbc914b16819237dcd8801d7e53f69e9719adecb3cc0e790c57e91ca4461c9548443b9a355c6010203c2040506a0c969a58f6f9095004c0177a6b47f451530cab38966a25cca5cb58f055542124e",
		wantPacket: &pong{
			To:         rpcEndpoint{net.ParseIP("2001:db8:85a3:8d3:1319:8a2e:370:7348"), 2222, 33338},
			ReplyTok:   common.Hex2Bytes("fbc914b16819237dcd8801d7e53f69e9719adecb3cc0e790c57e91ca4461c954"),
			Expiration: 1136239445,
			Rest:       []rlp.RawValue{{0xC6, 0x01, 0x02, 0x03, 0xC2, 0x04, 0x05}, {0x06}},
		},
	},
	{
		input: "c7c44041b9f7c7e41934417ebac9a8e1a4c6298f74553f2fcfdcae6ed6fe53163eb3d2b52e39fe91831b8a927bf4fc222c3902202027e5e9eb812195f95d20061ef5cd31d502e47ecb61183f74a504fe04c51e73df81f25c4d506b26db4517490103f84eb840ca634cae0d49acb401d8a4c6b6fe8c55b70d115bf400769cc1400f3258cd31387574077f301b421bc84df7266c44e9e6d569fc56be00812904767bf5ccd1fc7f8443b9a35582999983999999280dc62cc8255c73471e0a61da0c89acdc0e035e260add7fc0c04ad9ebf3919644c91cb247affc82b69bd2ca235c71eab8e49737c937a2c396",
		wantPacket: &findnode{
			Target:     MustHexID("ca634cae0d49acb401d8a4c6b6fe8c55b70d115bf400769cc1400f3258cd31387574077f301b421bc84df7266c44e9e6d569fc56be00812904767bf5ccd1fc7f"),
			Expiration: 1136239445,
			Rest:       []rlp.RawValue{{0x82, 0x99, 0x99}, {0x83, 0x99, 0x99, 0x99}},
		},
	},
	{
		input: "c679fc8fe0b8b12f06577f2e802d34f6fa257e6137a995f6f4cbfc9ee50ed3710faf6e66f932c4c8d81d64343f429651328758b47d3dbc02c4042f0fff6946a50f4a49037a72bb550f3a7872363a83e1b9ee6469856c24eb4ef80b7535bcf99c0004f9015bf90150f84d846321163782115c82115db8403155e1427f85f10a5c9a7755877748041af1bcd8d474ec065eb33df57a97babf54bfd2103575fa829115d224c523596b401065a97f74010610fce76382c0bf32f84984010203040101b840312c55512422cf9b8a4097e9a6ad79402e87a15ae909a4bfefa22398f03d20951933beea1e4dfa6f968212385e829f04c2d314fc2d4e255e0d3bc08792b069dbf8599020010db83c4d001500000000abcdef12820d05820d05b84038643200b172dcfef857492156971f0e6aa2c538d8b74010f8e140811d53b98c765dd2d96126051913f44582e8c199ad7c6d6819e9a56483f637feaac9448aacf8599020010db885a308d313198a2e037073488203e78203e8b8408dcab8618c3253b558d459da53bd8fa68935a719aff8b811197101a4b2b47dd2d47295286fc00cc081bb542d760717d1bdd6bec2c37cd72eca367d6dd3b9df738443b9a355010203b525a138aa34383fec3d2719a0",
		wantPacket: &neighbors{
			Nodes: []rpcNode{
				{
					ID:  MustHexID("3155e1427f85f10a5c9a7755877748041af1bcd8d474ec065eb33df57a97babf54bfd2103575fa829115d224c523596b401065a97f74010610fce76382c0bf32"),
					IP:  net.ParseIP("99.33.22.55").To4(),
					UDP: 4444,
					TCP: 4445,
				},
				{
					ID:  MustHexID("312c55512422cf9b8a4097e9a6ad79402e87a15ae909a4bfefa22398f03d20951933beea1e4dfa6f968212385e829f04c2d314fc2d4e255e0d3bc08792b069db"),
					IP:  net.ParseIP("1.2.3.4").To4(),
					UDP: 1,
					TCP: 1,
				},
				{
					ID:  MustHexID("38643200b172dcfef857492156971f0e6aa2c538d8b74010f8e140811d53b98c765dd2d96126051913f44582e8c199ad7c6d6819e9a56483f637feaac9448aac"),
					IP:  net.ParseIP("2001:db8:3c4d:15::abcd:ef12"),
					UDP: 3333,
					TCP: 3333,
				},
				{
					ID:  MustHexID("8dcab8618c3253b558d459da53bd8fa68935a719aff8b811197101a4b2b47dd2d47295286fc00cc081bb542d760717d1bdd6bec2c37cd72eca367d6dd3b9df73"),
					IP:  net.ParseIP("2001:db8:85a3:8d3:1319:8a2e:370:7348"),
					UDP: 999,
					TCP: 1000,
				},
			},
			Expiration: 1136239445,
			Rest:       []rlp.RawValue{{0x01}, {0x02}, {0x03}},
		},
	},
}

func TestForwardCompatibility(t *testing.T) {
	t.Skip("skipped while working on discovery v5")

	testkey, _ := crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	wantNodeID := PubkeyID(&testkey.PublicKey)

	for _, test := range testPackets {
		input, err := hex.DecodeString(test.input)
		if err != nil {
			t.Fatalf("invalid hex: %s", test.input)
		}
		var pkt ingressPacket
		if err := decodePacket(input, &pkt); err != nil {
			t.Errorf("did not accept packet %s\n%v", test.input, err)
			continue
		}
		if !reflect.DeepEqual(pkt.data, test.wantPacket) {
			t.Errorf("got %s\nwant %s", spew.Sdump(pkt.data), spew.Sdump(test.wantPacket))
		}
		if pkt.remoteID != wantNodeID {
			t.Errorf("got id %v\nwant id %v", pkt.remoteID, wantNodeID)
		}
	}
}

//dgrampipe是一个假的udp套接字。它将所有发送的数据报排队。
type dgramPipe struct {
	mu      *sync.Mutex
	cond    *sync.Cond
	closing chan struct{}
	closed  bool
	queue   [][]byte
}

func newpipe() *dgramPipe {
	mu := new(sync.Mutex)
	return &dgramPipe{
		closing: make(chan struct{}),
		cond:    &sync.Cond{L: mu},
		mu:      mu,
	}
}

//writetoudp将数据报排队。
func (c *dgramPipe) WriteToUDP(b []byte, to *net.UDPAddr) (n int, err error) {
	msg := make([]byte, len(b))
	copy(msg, b)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, errors.New("closed")
	}
	c.queue = append(c.queue, msg)
	c.cond.Signal()
	return len(b), nil
}

//readfromudp只是挂起，直到管道关闭。
func (c *dgramPipe) ReadFromUDP(b []byte) (n int, addr *net.UDPAddr, err error) {
	<-c.closing
	return 0, nil, io.EOF
}

func (c *dgramPipe) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		close(c.closing)
		c.closed = true
	}
	return nil
}

func (c *dgramPipe) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: testLocal.IP, Port: int(testLocal.UDP)}
}

func (c *dgramPipe) waitPacketOut() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	for len(c.queue) == 0 {
		c.cond.Wait()
	}
	p := c.queue[0]
	copy(c.queue, c.queue[1:])
	c.queue = c.queue[:len(c.queue)-1]
	return p
}
