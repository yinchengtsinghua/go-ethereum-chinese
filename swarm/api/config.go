
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

package api

import (
	"crypto/ecdsa"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/ens"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/pss"
	"github.com/ethereum/go-ethereum/swarm/services/swap"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

const (
	DefaultHTTPListenAddr = "127.0.0.1"
	DefaultHTTPPort       = "8500"
)

//单独的BZZ目录
//允许多个bzz节点并行运行
type Config struct {
//序列化/持久化字段
	*storage.FileStoreParams
	*storage.LocalStoreParams
	*network.HiveParams
	Swap *swap.LocalProfile
	Pss  *pss.PssParams
 /*网络.syncparams
 合同通用地址
 ENSROOT公用地址
 ensapis[]字符串
 路径字符串
 listenaddr字符串
 端口字符串
 公钥字符串
 bzzkey字符串
 nodeid字符串
 网络ID uint64
 可旋转吊杆
 同步启用布尔值
 同步kipcheck bool
 交付kipcheck bool
 maxstreampeerservers int
 轻节点启用bool
 同步更新延迟时间。持续时间
 SWAPPI字符串
 CORS弦
 bzzaccount字符串
 私钥*ecdsa.privatekey
}

//创建一个默认配置，所有参数都设置为默认值
func newconfig（）（c*config）

 C=和CONFIG {
  localstoreparams:storage.newdefaultlocalstoreparams（），
  filestoreparams:storage.newfilestoreparams（），
  hiveParams:network.newhiveParams（），
  //同步参数：network.newDefaultSyncParams（），
  swap:swap.newdefaultswapparams（），
  pss:pss.newpssparams（），
  listenaddr:默认httplistenaddr，
  端口：默认httpport，
  路径：node.defaultdatadir（），
  ENSAPI：无，
  ensroot:ens.testnetaddress，网址：
  networkid:network.defaultnetworkid，
  SWAPENABLED:错误，
  已启用同步：真，
  SyncingSkipCheck:错误，
  MaxstreamPeerServers:10000台，
  DeliverySkipCheck:是，
  同步更新延迟：15*次。秒，
  斯瓦帕皮：“，
 }

 返回
}

//完成后需要初始化一些配置参数
//配置构建阶段已完成（例如，由于覆盖标志）
func（c*config）init（prvkey*ecdsa.privatekey）

 地址：=crypto.pubkeytoAddress（prvkey.publickey）
 c.path=filepath.join（c.path，“bzz-”+common.bytes2hex（address.bytes（）））
 错误：=os.mkdirall（c.path，os.modeperm）
 如果犯错！= nIL{
  log.error（fmt.sprintf（“创建根Swarm数据目录时出错：%v”，err））。
  返回
 }

 pubkey：=crypto.fromecdsapub（&prvkey.publickey）
 pubkeyhex：=common.tohex（pubkey）
 keyHex：=crypto.keccak256hash（pubkey.hex（））

 c.publickey=pubKeyHex
 c.bzzkey=六角键
 c.nodeid=enode.pubKeyToIDv4（&prvkey.publickey）.string（））

 如果C.SWAPENABLED
  c.swap.init（c.contract，prvkey）
 }

 c.privatekey=prvkey
 c.localstoreparams.init（c.path）初始化
 c.localstoreparams.basekey=common.fromhex（keyhex）

 c.pss=c.pss.withprivatekey（c.privatekey）
}

func（c*config）shiftprivatekey（）（privkey*ecdsa.privatekey）
 如果是C.privatekey！= nIL{
  privkey=c.privatekey
  c.privatekey=无
 }
 返回私钥
}
