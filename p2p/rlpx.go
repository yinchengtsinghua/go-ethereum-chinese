
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2015 Go Ethereum作者
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
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	mrand "math/rand"
	"net"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/bitutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/golang/snappy"
	"golang.org/x/crypto/sha3"
)

const (
	maxUint24 = ^uint32(0) >> 8

sskLen = 16 //指定最大共享密钥长度（pubkey）/2
sigLen = 65 //椭圆形S256
pubLen = 64 //512位pubkey，未压缩表示，无格式字节
shaLen = 32 //哈希长度（用于nonce等）

	authMsgLen  = sigLen + shaLen + pubLen + shaLen + 1
	authRespLen = pubLen + shaLen + 1

 /*eSoverhead=65/*pubkey*/+16/*iv*/+32/*mac*/

 encauthmsglen=authmsglen+eciesoverhead//加密的pre-eip-8发起程序握手大小
 encauthresplen=authresplen+eciesoverhead//加密的pre-eip-8握手回复的大小

 //加密握手和协议的总超时时间
 //双向握手。
 握手超时=5*次。秒

 //这是发送断开连接原因的超时。
 //这比通常的超时时间短，因为我们不希望
 //等待连接是否损坏。
 discWriteTimeout=1*时间。秒
）

//如果解压缩的消息长度超过
//允许的24位（即长度大于等于16MB）。
var errplanmessagetoolarge=errors.new（“消息长度>=16MB”）

//rlpx是实际（非测试）连接使用的传输协议。
//它用锁和读/写截止时间包装帧编码器。
RLPX型结构
 康德网络

 rmu，wmu同步.mutex
 rw*rlpxframerw
}

func newrlpx（fd net.conn）传输
 fd.setDeadline（time.now（）.add（handshakeTimeout））。
 返回&rlpx fd:fd
}

func（t*rlpx）readmsg（）（msg，错误）
 锁定（）
 延迟t.rmu.unlock（）
 t.fd.setreadDeadline（time.now（）.add（framereadTimeout））。
 返回t.rw.readmsg（）。
}

func（t*rlpx）writemsg（msg msg）错误
 T.WMU.
 延迟t.wmu.unlock（）
 t.fd.setWriteDeadline（time.now（）.add（frameWriteTimeout））。
 返回t.rw.writemsg（msg）
}

func（t*rlpx）关闭（err错误）
 T.WMU.
 延迟t.wmu.unlock（）
 //如果可能，告诉远端为什么要断开连接。
 如果T.RW！= nIL{
  如果R，则OK：=错误（不合理）；OK&&R！=DiscNetworkError
   //rlpx尝试向断开连接的对等端发送discreason
   //如果连接为net.pipe（内存模拟）
   //它永远挂起，因为net.pipe不实现
   //写入截止时间。因为这只会试图发送
   //如果没有错误，则显示断开原因消息。
   if err：=t.fd.setWriteDeadline（time.now（）.add（discWriteTimeout））；err==nil
    发送项（t.rw、discmsg、r）
   }
  }
 }
 T.F.C.（）
}

func（t*rlpx）doprotochandshake（our*protochandshake）（their*protochandshake，err error）
 //写我们的握手是同时进行的，我们更喜欢
 //返回握手读取错误。如果远端
 //有正当理由提前断开我们的连接，我们应该将其返回
 //作为错误，以便在其他地方跟踪它。
 werr：=make（chan错误，1）
 go func（）werr<-send（t.rw，handshakemsg，our）（）
 如果他们的，err=readProtocolHandshake（t.rw，我们的）；err！= nIL{
  <-werr//确保写入也终止
  返回零
 }
 如果错误：=<-werr；err！= nIL{
  返回nil，fmt.errorf（“写入错误：%v”，err）
 }
 //如果协议版本支持快速编码，则立即升级
 t.rw.snappy=their.version>=snappyrotocolversion

 归还他们的，零
}

func readprotocolhandshake（rw msgreader，our*protochandshake）（*protochandshake，error）
 msg，err:=rw.readmsg（）。
 如果犯错！= nIL{
  返回零
 }
 如果消息大小>baseprotocolmaxmsgsize
  返回nil，fmt.errorf（“消息太大”）
 }
 如果msg.code==discmsg
  //根据协议握手有效之前断开连接
  //spec，如果post handshake检查失败，我们会自行发送。
  //但是我们不能直接返回原因，因为它被回送了
  //否则返回。改为用绳子把它包起来。
  VaR原因[1]不一致
  rlp.decode（消息有效负载和原因）
  返回零，原因[0]
 }
 如果是MSG，代码！=握手
  返回nil，fmt.errorf（“预期握手，得到%x”，消息代码）
 }
 var-hs协议握手
 如果错误：=msg.decode（&hs）；错误！= nIL{
  返回零
 }
 如果莱恩（H.ID）！= 64乘！bitutil.testBytes（hs.id）_
  返回零，DiscInvalidIdentity
 }
 返回与HS
}

//doenchandshake使用authenticated运行协议握手
/ /消息。协议握手是第一条经过身份验证的消息
//并验证加密握手是否“有效”以及
//远程端实际上提供了正确的公钥。
func（t*rlpx）doenchandshake（prv*ecdsa.privatekey，dial*ecdsa.publickey）（*ecdsa.publickey，error）
 var
  证券交易秘密
  错误率
 ）
 如果刻度盘=零
  sec，err=receiverenchandshake（t.fd，prv）
 }否则{
  sec，err=initiatorenchandshake（t.fd，prv，dial）
 }
 如果犯错！= nIL{
  返回零
 }
 T.WMU.
 t.rw=newrlpxframerw（t.fd，秒）
 解锁（）
 返回sec.remote.exportecdsa（），nil
}

//enchandshake包含加密握手的状态。
类型enchandshake结构
 发起人bool
 remote*ecies.publickey//远程pubk
 initnoce，respnoce[]字节//nonce
 randomprivkey*ecies.privatekey//ecdhe随机
 remoterandompub*ecies.publickey//ecdhe random pubk
}

//机密表示连接机密
//在加密握手期间协商。
类型机密结构
 远程*ecies.publickey
 aes，mac[]字节
 出口MAC，入口MAC哈希.hash
 标记[]字节
}

//rlpx v4握手认证（在eip-8中定义）。
类型authmsgv4结构
 gotplan bool//读包是否有纯格式。

 签名[siglen]字节
 initiatorSubkey[publen]字节
 nonce[shalen]字节
 版本uint

 //忽略其他字段（向前兼容）
 其余[]rlp.rawvalue`rlp:“tail”`
}

//rlpx v4握手响应（在eip-8中定义）。
键入authrespv4 struct_
 randompubkey[publen]字节
 nonce[shalen]字节
 版本uint

 //忽略其他字段（向前兼容）
 其余[]rlp.rawvalue`rlp:“tail”`
}

//在握手完成后调用secrets。
//从握手值中提取连接机密。
func（h*enchandshake）机密（auth，authresp[]byte）（机密，错误）
 ecdhesecret，err:=h.randomprivkey.generateshared（h.remoterrandompub，ssklen，ssklen）
 如果犯错！= nIL{
  返回机密，错误
 }

 //从临时密钥协议派生基本机密
 sharedSecret：=crypto.keccak256（ecdhesecret，crypto.keccak256（h.responce，h.initonce））。
 aesecret：=crypto.keccak256（ecdhesecret，sharedsecret）
 S: =秘密{
  遥控器：H.遥控器，
  伊丝：伊丝密，
  mac:crypto.keccak256（ecdhesecret，aesecret）
 }

 //为macs设置sha3实例
 mac1：=sha3.newlegacykeccak256（）。
 mac1.write（xor（s.mac，h.respnce））。
 mac1.写入（auth）
 mac2：=sha3.newlegacykeccak256（）。
 mac2.write（xor（s.mac，h.initonce））。
 MAC2.写入（authresp）
 如果H.发起人
  S.egressmac，S.ingressmac=mac1，mac2
 }否则{
  S.egressmac，S.ingressmac=mac2，mac1
 }

 返回零
}

//StaticSharedSecret返回静态共享机密，结果
//本地和远程静态节点密钥之间的密钥协议。
func（h*enchandshake）staticsharedsecret（prv*ecdsa.privatekey）（[]字节，错误）
 返回ecies.importecdsa（prv）.generateshared（h.remote、ssklen、ssklen）
}

//initiatorenchandshake在conn上协商会话令牌。
//应该在连接的拨号端调用它。
/ /
//prv是本地客户端的私钥。
func initiatorenchandshake（conn io.readwriter，prv*ecdsa.privatekey，remote*ecdsa.publickey）（s secrets，err error）
 h：=&enchandshake发起者：真，远程：指定。导入ecdsapublic（远程）
 authmsg，错误：=h.makeauthmsg（prv）
 如果犯错！= nIL{
  返回S
 }
 authpacket，错误：=sealeip8（authmsg，h）
 如果犯错！= nIL{
  返回S
 }
 如果，err=conn.write（authpacket）；err！= nIL{
  返回S
 }

 AuthRespMsg：=新建（AuthRespv4）
 AuthRespPacket，错误：=readHandshakemsg（AuthRespMsg，EnauthRespLen，prv，conn）
 如果犯错！= nIL{
  返回S
 }
 如果错误：=H.HandleAuthResp（AuthRespMsg）；错误！= nIL{
  返回S
 }
 返回h.secrets（authpacket、authrespacket）
}

//makeauthmsg创建启动器握手消息。
func（h*enchandshake）makeauthmsg（prv*ecdsa.privatekey）（*authmsgv4，错误）
 //生成随机发起程序nonce。
 h.initonce=生成（[]字节，shalen）
 错误：=rand.read（h.initonce）
 如果犯错！= nIL{
  返回零
 }
 //为ECDH生成随机密钥对to。
 h.randomprivkey，err=ecies.generatekey（rand.reader，crypto.s256（），nil）
 如果犯错！= nIL{
  返回零
 }

 //为已知消息签名：静态共享机密^nonce
 令牌，错误：=h.staticsharedsecret（prv）
 如果犯错！= nIL{
  返回零
 }
 有符号：=xor（token，h.initonce）
 签名，错误：=crypto.sign（signed，h.randomprivkey.exportecdsa（））
 如果犯错！= nIL{
  返回零
 }

 消息：=new（authmsgv4）
 副本（消息签名[：]，签名）
 复制（msg.initiatorpubkey[：]，crypto.fromecdsapub（&prv.publickey）[1:]
 复制（msg.nonce[：]，h.initonce）
 版本＝4
 返回MSG，NIL
}

func（h*enchandshake）handleauthresp（msg*authresp4）（错误）
 h.respnce=msg.nonce[：]
 h.remoterandompub，err=importpublickey（msg.randompubkey[：]）
 返回错误
}

//receiverenchandshake在conn上协商会话令牌。
//应该在连接的侦听端调用它。
/ /
//prv是本地客户端的私钥。
func receiverenchandshake（conn io.readwriter，prv*ecdsa.privatekey）（s secrets，err error）
 authmsg：=新建（authmsgv4）
 authpacket，错误：=readhandshakemsg（authmsg，encauthmsglen，prv，conn）
 如果犯错！= nIL{
  返回S
 }
 H：=新（Enchandshake）
 如果错误：=h.handleauthmsg（authmsg，prv）；错误！= nIL{
  返回S
 }

 AuthRespMsg，错误：=H.MakeAuthResp（）。
 如果犯错！= nIL{
  返回S
 }
 var authresppacket[]字节
 如果authmsg.gotplan
  AuthRespPacket，err=AuthRespMsg.SealPlain（H）
 }否则{
  AuthRespPacket，err=SealeIP8（AuthRespMsg，H）
 }
 如果犯错！= nIL{
  返回S
 }
 如果，err=conn.write（authresppacket）；err！= nIL{
  返回S
 }
 返回h.secrets（authpacket、authrespacket）
}

func（h*enchandshake）handleauthmsg（msg*authmsgv4，prv*ecdsa.privatekey）错误
 //导入远程标识。
 rpub，err：=importpublickey（msg.initiatorpubkey[：]）
 如果犯错！= nIL{
  返回错误
 }
 h.initonce=msg.nonce[：]
 远程遥控器

 //为ECDH生成随机密钥对。
 //如果已经设置了私钥，则使用它而不是生成一个（用于测试）。
 如果h.randomprivkey==nil
  h.randomprivkey，err=ecies.generatekey（rand.reader，crypto.s256（），nil）
  如果犯错！= nIL{
   返回错误
  }
 }

 //检查签名。
 令牌，错误：=h.staticsharedsecret（prv）
 如果犯错！= nIL{
  返回错误
 }
 signedsg：=xor（token，h.initonce）
 remoteRandoPub，错误：=secp256k1.recoverpubkey（signedsg，msg.signature[：]）
 如果犯错！= nIL{
  返回错误
 }
 H.RemoteRandompub，u=导入公共密钥（RemoteRandompub）
 返回零
}

func（h*enchandshake）makeauthresp（）（msg*authresp4，err error）
 //生成随机nonce。
 h.responce=make（[]字节，shalen）
 如果，err=rand.read（h.respnce）；err！= nIL{
  返回零
 }

 msg=新建（authrespv4）
 复制（msg.nonce[：]，h.respnce）
 复制（msg.randompubkey[：]，exportpubkey（&h.randomprivkey.publickey））
 版本＝4
 返回MSG，NIL
}

func（msg*authmsgv4）sealplain（h*enchandshake）（[]字节，错误）
 buf：=make（[]字节，authmsglen）
 n：=副本（buf，msg.签名[：]）
 n+=复制（buf[n:]，crypto.keccak256（exportpubkey（&h.randomprivkey.publickey）））
 n+=复制（buf[n:]，msg.initiatorSubkey[：]）
 n+=复制（buf[n:]，msg.nonce[：]）
 buf[n]=0//令牌标志
 返回ecies.encrypt（rand.reader、h.remote、buf、nil、nil）
}

func（msg*authmsgv4）decodeplain（input[]byte）
 n：=复制（消息签名[：]，输入）
 n+=shalen//跳过sha3（发起人短暂发布）
 n+=复制（msg.initiatorSubkey[：]，输入[n:]
 复制（msg.nonce[：]，输入[n:]）
 版本＝4
 msg.gotplan=真
}

func（msg*authrespv4）sealplain（hs*enchandshake）（[]字节，错误）
 buf：=make（[]字节，authresplen）
 n：=复制（buf，msg.randombkey[：]）
 复制（buf[n:]，msg.nonce[：]）
 返回ecies.encrypt（rand.reader、hs.remote、buf、nil、nil）
}

func（msg*authrespv4）decodeplain（input[]byte）
 n：=复制（msg.randompubkey[：]，输入）
 复制（msg.nonce[：]，输入[n:]）
 版本＝4
}

var padspace=make（[]字节，300）

func sealeip8（msg interface，h*enchandshake）（[]字节，错误）
 buf：=新建（bytes.buffer）
 如果错误：=rlp.encode（buf，msg）；错误！= nIL{
  返回零
 }
 //用随机数据量填充。数量至少需要100个字节才能
 //该消息可与EIP-8之前的握手区分开来。
 焊盘：=padspace[：mrand.intn（len（padspace）-100）+100]
 BUF.写（PAD）
 前缀：=make（[]字节，2）
 binary.bigendian.putuint16（前缀，uint16（buf.len（）+eciesoverhead））

 enc，err：=ecies.encrypt（rand.reader，h.remote，buf.bytes（），nil，prefix）
 返回append（prefix，enc…），err
}

类型普通解码器接口
 decodeplain（[]字节）
}

func readhandshakemsg（msg plaindecoder，plainsize int，prv*ecdsa.privatekey，r io.reader）（[]字节，错误）
 buf：=make（[]字节，普通大小）
 如果，错误：=io.readfull（r，buf）；错误！= nIL{
  返回BUF
 }
 //尝试解码pre-eip-8“plain”格式。
 键：=ecies.importecdsa（prv）
 如果是dec，则错误：=key.decrypt（buf，nil，nil）；err==nil
  解码原稿消息（DEC）
  返回BUF，NIL
 }
 //可以是EIP-8格式，试试看。
 前缀：=buf[：2]
 大小：=binary.bigendian.uint16（前缀）
 如果尺寸<uint16（plainsize）
  返回buf，fmt.errorf（“大小下溢，至少需要%d字节”，plainsize）
 }
 buf=append（buf，make（[]byte，size-uint16（plainsize）+2）…）
 如果uu，错误：=io.readfull（r，buf[plainsize:]）；错误！= nIL{
  返回BUF
 }
 dec，err：=key.decrypt（buf[2:]，nil，前缀）
 如果犯错！= nIL{
  返回BUF
 }
 //此处不能使用rlp.decodebytes，因为它拒绝
 //尾随数据（向前兼容）。
 S：=rlp.newstream（bytes.newreader（dec），0）
 返回buf，s.decode（msg）
}

//importpublickey取消标记512位公钥。
func importpublickey（pubkey[]byte）（*ecies.publickey，error）
 var pubkey65[]字节
 交换长度（pubkey）
 案例64：
  //添加“uncompressed key”标志
  pubkey65=append（[]字节0x04，pubkey…）
 案例65：
  pubKey65=发布键
 违约：
  返回nil，fmt.errorf（“无效的公钥长度%v（预期64/65）”，len（pubkey））
 }
 //TODO:更少的无意义转换
 pub，err：=crypto.unmashalpubkey（pubkey65）
 如果犯错！= nIL{
  返回零
 }
 退货特定进口适用性（pub），无
}

func exportpubkey（pub*ecies.publickey）[]字节
 如果Pub＝nI{
  恐慌（“nil pubkey”）
 }
 返回Elliptic.Marshal（pub.curve，pub.x，pub.y）[1:]
}

func xor（一个，另一个[]字节）（xor[]字节）
 xor=make（[]byte，len（one））。
 对于i：=0；i<len（one）；i++
  xor[i]=一个[i]^另一个[i]
 }
 返回异或
}

var
 //用于代替实际帧头数据。
 //TODO:当msg包含协议类型代码时替换此项。
 ZeroHeader=[]字节0xc2、0x80、0x80_
 //十六个零字节
 zero16=make（[]字节，16）
）

//rlpxframerw实现了rlpx framework的简化版本。
//不支持分块消息，所有头等于
//ZooHead。
/ /
//rlpxframerw对于从多个goroutine并发使用不安全。
类型rlpxframerw struct_
 连接IO.readwriter
 加密流
 DEC密码流

 maccipher cipher.block
 Egressmac哈希.hash
 入口MAC哈希.hash

 快活布尔
}

func newrlpxframerw（conn io.readwriter，s secrets）*rlpxframerw_
 macc，err：=aes.newcipher（s.mac）
 如果犯错！= nIL{
  panic（“无效的MAC机密：”+err.error（））
 }
 ENCC，错误：=aes.newcipher（s.aes）
 如果犯错！= nIL{
  panic（“无效的aes秘密：”+err.error（））
 }
 //我们对aes使用全零IV，因为使用的密钥
 //因为加密是短暂的。
 iv：=make（[]字节，encc.blocksize（））
 返回&rlpxframerw_
  连接件：连接件，
  Enc:Cipher.Newctr（Encc，IV），第
  DEC:cipher.newctr（encc，iv）
  麦克西弗：麦克西弗，
  白鹭：白鹭，
  入口MAC:S.IngressMAC，
 }
}

func（rw*rlpxframerw）writemsg（msg msg）错误
 ptype，：=rlp.encodetobytes（消息代码）

 //如果启用了snappy，则立即压缩消息
 如果RW.SNAPPY {
  如果消息大小>maxuint24
   返回errplanmessagetoolarge
  }
  有效载荷，：=ioutil.readall（msg.payload）
  有效载荷=快速编码（零，有效载荷）

  msg.payload=bytes.newreader（有效负载）
  msg.size=uint32（len（有效载荷）
 }
 //写头
 headbuf：=make（[]字节，32）
 fsize：=uint32（len（ptype））+消息大小
 如果fsize>maxuint24
  返回errors.new（“消息大小溢出uint24”）
 }
 putint24（fsize，headbuf）//todo:检查溢出
 复制（headbuf[3:]，zeroheader）
 rw.enc.xorkeystream（headbuf[：16]，headbuf[：16]）//上半部分现在已加密

 //写入头MAC
 副本（headbuf[16:]，updatemac（rw.egissmac，rw.maccipher，headbuf[：16]））
 如果uux，错误：=rw.conn.write（headbuf）；错误！= nIL{
  返回错误
 }

 //写入加密帧，更新出口MAC哈希
 //写入conn的数据。
 tee：=cipher.streamwriter s:rw.enc，w:io.multiwriter（rw.conn，rw.egissmac）
 如果ux，err:=tee.write（ptype）；err！= nIL{
  返回错误
 }
 如果uu，错误：=io.copy（tee，msg.payload）；错误！= nIL{
  返回错误
 }
 如果填充：=fsize%16；填充>0
  如果u，err：=tee.write（zero16[：16 padding]）；err！= nIL{
   返回错误
  }
 }

 //写入帧mac。出口MAC哈希是最新的，因为
 //框架内容也写入了它。
 fmacseed：=rw.egissmac.sum（零）
 mac：=更新mac（rw.egissmac、rw.maccipher、fmacseed）
 _uuErr：=rw.conn.write（mac）
 返回错误
}

func（rw*rlpxframerw）readmsg（）（msg，err error）
 //读取头
 headbuf：=make（[]字节，32）
 如果，错误：=io.readfull（rw.conn，headbuf）；错误！= nIL{
  返回MSG
 }
 //验证头MAC
 shouldmac：=updateMac（rw.ingressmac，rw.maccipher，headbuf[：16]）
 如果！hmac.相等（shouldmac，headbuf[16:）
  返回msg，errors.new（“错误的header mac”）
 }
 rw.dec.xorkeystream（headbuf[：16]，headbuf[：16]）//上半部分现在被解密
 fsize：=readInt24（headbuf）
 //暂时忽略协议类型

 //读取帧内容
 var rsize=fsize//帧大小四舍五入到16字节边界
 如果填充：=fsize%16；填充>0
  rsize+=16-填充
 }
 framebuf：=make（[]字节，rsize）
 如果uux，错误：=io.readfull（rw.conn，framebuf）；错误！= nIL{
  返回MSG
 }

 //读取并验证帧MAC。我们可以重复使用headbuf。
 rw.ingressmac.write（framebuf）
 fmacseed：=rw.ingressmac.sum（零）
 如果uu，错误：=io.readfull（rw.conn，headbuf[：16]）；错误！= nIL{
  返回MSG
 }
 shouldmac=updateMac（rw.ingressmac，rw.maccipher，fmacseed）
 如果！hmac.equal（shouldmac，headbuf[：16]）；
  返回msg，errors.new（“坏帧MAC”）
 }

 //解密帧内容
 rw.dec.xorkeystream（framebuf，framebuf）代码

 //解码消息代码
 内容：=bytes.newreader（framebuf[：fsize]）
 如果错误：=rlp.decode（content，&msg.code）；错误！= nIL{
  返回MSG
 }
 msg.size=uint32（content.len（））
 msg.payload=内容

 //如果启用了snappy，则验证并解压缩消息
 如果RW.SNAPPY {
  有效负载，错误：=ioutil.readall（msg.payload）
  如果犯错！= nIL{
   返回MSG
  }
  大小，错误：=snappy.decodedlen（有效负载）
  如果犯错！= nIL{
   返回MSG
  }
  如果大小>int（maxuint24）
   返回消息，errplanmessagetoolarge
  }
  有效载荷，err=snappy.decode（nil，有效载荷）
  如果犯错！= nIL{
   返回MSG
  }
  msg.size，msg.payload=uint32（大小），bytes.newreader（有效负载）
 }
 返回MSG，NIL
}

//updateMac使用加密种子重新设置给定哈希。
//返回种子设定后哈希和的前16个字节。
func updatemac（mac hash.hash，block cipher.block，seed[]byte）[]byte_
 aesbuf：=make（[]字节，aes.blocksize）
 block.encrypt（aesbuf，mac.sum（nil））。
 对于i：=范围aesbuf
  aesbuf[i]^=种子[i]
 }
 书写（aesbuf）
 返回mac.sum（nil）[：16]
}

func readint24（b[]字节）uint32
 返回uint32（b[2]）uint32（b[1]）<<8_uint32（b[0]）<<16
}

func putint24（v uint32，b[]字节）
 B[0]=字节（V>>16）
 B[1]=字节（V>>8）
 B〔2〕＝字节（V）
}
