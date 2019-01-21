
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

package node

import (
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/accounts/usbwallet"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
datadirPrivateKey      = "nodekey"            //datadir中指向节点私钥的路径
datadirDefaultKeyStore = "keystore"           //到密钥库的datadir中的路径
datadirStaticNodes     = "static-nodes.json"  //到静态节点列表的datadir中的路径
datadirTrustedNodes    = "trusted-nodes.json" //datadir中到受信任节点列表的路径
datadirNodeDatabase    = "nodes"              //datadir中存储节点信息的路径
)

//config表示一个小的配置值集合，用于微调
//协议栈的P2P网络层。这些值可以进一步扩展为
//所有注册服务。
type Config struct {
//名称设置节点的实例名称。它不能包含/字符，并且
//在devp2p节点标识符中使用。geth的实例名为“geth”。如果没有
//如果指定了值，则使用当前可执行文件的基名称。
	Name string `toml:"-"`

//如果设置了userIdent，则用作devp2p节点标识符中的附加组件。
	UserIdent string `toml:",omitempty"`

//版本应设置为程序的版本号。它被使用
//在devp2p节点标识符中。
	Version string `toml:"-"`

//datadir是节点应用于任何数据存储的文件系统文件夹。
//要求。配置的数据目录将不会直接与共享
//注册的服务，而这些服务可以使用实用工具方法来创建/访问
//数据库或平面文件。这将启用可以完全驻留的临时节点
//在记忆中。
	DataDir string

//对等网络的配置。
	P2P p2p.Config

//keystoredir是包含私钥的文件系统文件夹。目录可以
//被指定为相对路径，在这种情况下，它是相对于
//当前目录。
//
//如果keystoredir为空，则默认位置是的“keystore”子目录。
//DATADIR如果未指定datadir且keystoredir为空，则为临时目录
//由new创建并在节点停止时销毁。
	KeyStoreDir string `toml:",omitempty"`

//uselightweightkdf降低了密钥存储的内存和CPU要求
//以牺牲安全为代价加密kdf。
	UseLightweightKDF bool `toml:",omitempty"`

//nousb禁用硬件钱包监控和连接。
	NoUSB bool `toml:",omitempty"`

//ipcpath是放置ipc端点的请求位置。如果路径是
//一个简单的文件名，它放在数据目录（或根目录）中
//窗口上的管道路径），但是如果它是可解析的路径名（绝对或
//相对），然后强制执行该特定路径。空路径禁用IPC。
	IPCPath string `toml:",omitempty"`

//http host是启动HTTP RPC服务器的主机接口。如果这样
//字段为空，不会启动HTTP API终结点。
	HTTPHost string `toml:",omitempty"`

//http port是启动HTTP RPC服务器的TCP端口号。这个
//默认的零值是/有效的，将随机选择端口号（有用
//对于季节性节点）。
	HTTPPort int `toml:",omitempty"`

//httpcors是要发送到请求的跨源资源共享头
//客户。请注意，CORS是一种浏览器强制的安全性，它完全
//对自定义HTTP客户端无效。
	HTTPCors []string `toml:",omitempty"`

//httpvirtualhosts是传入请求上允许的虚拟主机名列表。
//默认为“localhost”。使用它可以防止像
//DNS重新绑定，它通过简单地伪装成在同一个sop中绕过了sop
//起源。这些攻击不使用CORS，因为它们不是跨域的。
//通过显式检查主机头，服务器将不允许请求
//针对具有恶意主机域的服务器。
//直接使用IP地址的请求不受影响
	HTTPVirtualHosts []string `toml:",omitempty"`

//http modules是要通过HTTP RPC接口公开的API模块列表。
//如果模块列表为空，则所有指定为public的RPC API端点都将
//暴露的。
	HTTPModules []string `toml:",omitempty"`

//HTTPTimeouts允许自定义HTTP RPC使用的超时值
//接口。
	HTTPTimeouts rpc.HTTPTimeouts

//wshost是启动WebSocket RPC服务器的主机接口。如果
//此字段为空，将不启动WebSocket API终结点。
	WSHost string `toml:",omitempty"`

//wsport是启动WebSocket RPC服务器的TCP端口号。这个
//默认的零值是/有效的，将随机选择一个端口号（用于
//短暂的节点）。
	WSPort int `toml:",omitempty"`

//wsorigins是接受WebSocket请求的域列表。请
//注意，服务器只能根据客户机发送的HTTP请求进行操作，并且
//无法验证请求头的有效性。
	WSOrigins []string `toml:",omitempty"`

//wsmodules是要通过WebSocket RPC接口公开的API模块列表。
//如果模块列表为空，则所有指定为public的RPC API端点都将
//暴露的。
	WSModules []string `toml:",omitempty"`

//wsexposeall通过websocket rpc接口公开所有api模块，而不是
//而不仅仅是公众。
//
//*警告*仅当节点在受信任的网络中运行时设置此选项，显示
//对不受信任用户的私有API是一个主要的安全风险。
	WSExposeAll bool `toml:",omitempty"`

//logger是用于p2p.server的自定义记录器。
	Logger log.Logger `toml:",omitempty"`

	staticNodesWarning     bool
	trustedNodesWarning    bool
	oldGethResourceWarning bool
}

//ipc endpoint根据配置的值解析IPC端点，考虑
//帐户设置数据文件夹以及我们当前的指定平台
//继续运行。
func (c *Config) IPCEndpoint() string {
//如果未启用仪表板组合仪表，则短路
	if c.IPCPath == "" {
		return ""
	}
//在窗户上，我们只能使用普通的顶层管道。
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(c.IPCPath, `\\.\pipe\`) {
			return c.IPCPath
		}
		return `\\.\pipe\` + c.IPCPath
	}
//将名称解析为数据目录完整路径，否则
	if filepath.Base(c.IPCPath) == c.IPCPath {
		if c.DataDir == "" {
			return filepath.Join(os.TempDir(), c.IPCPath)
		}
		return filepath.Join(c.DataDir, c.IPCPath)
	}
	return c.IPCPath
}

//nodedb返回发现节点数据库的路径。
func (c *Config) NodeDB() string {
	if c.DataDir == "" {
return "" //短暂的
	}
	return c.ResolvePath(datadirNodeDatabase)
}

//defaultipcendpoint返回默认情况下使用的IPC路径。
func DefaultIPCEndpoint(clientIdentifier string) string {
	if clientIdentifier == "" {
		clientIdentifier = strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
		if clientIdentifier == "" {
			panic("empty executable name")
		}
	}
	config := &Config{DataDir: DefaultDataDir(), IPCPath: clientIdentifier + ".ipc"}
	return config.IPCEndpoint()
}

//http endpoint基于配置的主机接口解析HTTP端点
//和端口参数。
func (c *Config) HTTPEndpoint() string {
	if c.HTTPHost == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", c.HTTPHost, c.HTTPPort)
}

//default http endpoint返回默认情况下使用的HTTP端点。
func DefaultHTTPEndpoint() string {
	config := &Config{HTTPHost: DefaultHTTPHost, HTTPPort: DefaultHTTPPort}
	return config.HTTPEndpoint()
}

//wsendpoint基于配置的主机接口解析WebSocket终结点
//和端口参数。
func (c *Config) WSEndpoint() string {
	if c.WSHost == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", c.WSHost, c.WSPort)
}

//defaultwsendpoint返回默认情况下使用的WebSocket端点。
func DefaultWSEndpoint() string {
	config := &Config{WSHost: DefaultWSHost, WSPort: DefaultWSPort}
	return config.WSEndpoint()
}

//nodename返回devp2p节点标识符。
func (c *Config) NodeName() string {
	name := c.name()
//向后兼容性：以前的版本使用的标题是“geth”，请保留。
	if name == "geth" || name == "geth-testnet" {
		name = "Geth"
	}
	if c.UserIdent != "" {
		name += "/" + c.UserIdent
	}
	if c.Version != "" {
		name += "/v" + c.Version
	}
	name += "/" + runtime.GOOS + "-" + runtime.GOARCH
	name += "/" + runtime.Version()
	return name
}

func (c *Config) name() string {
	if c.Name == "" {
		progname := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
		if progname == "" {
			panic("empty executable name, set Config.Name")
		}
		return progname
	}
	return c.Name
}

//对于“geth”实例，这些资源的解析方式不同。
var isOldGethResource = map[string]bool{
	"chaindata":          true,
	"nodes":              true,
	"nodekey":            true,
"static-nodes.json":  false, //没有警告，因为他们有
"trusted-nodes.json": false, //单独警告。
}

//resolvepath解析实例目录中的路径。
func (c *Config) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if c.DataDir == "" {
		return ""
	}
//向后兼容性：确保创建了数据目录文件
//如果存在，则使用by geth 1.4。
	if warn, isOld := isOldGethResource[path]; isOld {
		oldpath := ""
		if c.name() == "geth" {
			oldpath = filepath.Join(c.DataDir, path)
		}
		if oldpath != "" && common.FileExist(oldpath) {
			if warn {
				c.warnOnce(&c.oldGethResourceWarning, "Using deprecated resource file %s, please move this file to the 'geth' subdirectory of datadir.", oldpath)
			}
			return oldpath
		}
	}
	return filepath.Join(c.instanceDir(), path)
}

func (c *Config) instanceDir() string {
	if c.DataDir == "" {
		return ""
	}
	return filepath.Join(c.DataDir, c.name())
}

//node key检索当前配置的节点私钥，检查
//首先是任何手动设置的键，返回到配置的
//数据文件夹。如果找不到密钥，则生成一个新的密钥。
func (c *Config) NodeKey() *ecdsa.PrivateKey {
//使用任何特定配置的密钥。
	if c.P2P.PrivateKey != nil {
		return c.P2P.PrivateKey
	}
//如果没有使用datadir，则生成临时密钥。
	if c.DataDir == "" {
		key, err := crypto.GenerateKey()
		if err != nil {
			log.Crit(fmt.Sprintf("Failed to generate ephemeral node key: %v", err))
		}
		return key
	}

	keyfile := c.ResolvePath(datadirPrivateKey)
	if key, err := crypto.LoadECDSA(keyfile); err == nil {
		return key
	}
//找不到持久密钥，生成并存储新密钥。
	key, err := crypto.GenerateKey()
	if err != nil {
		log.Crit(fmt.Sprintf("Failed to generate node key: %v", err))
	}
	instanceDir := filepath.Join(c.DataDir, c.name())
	if err := os.MkdirAll(instanceDir, 0700); err != nil {
		log.Error(fmt.Sprintf("Failed to persist node key: %v", err))
		return key
	}
	keyfile = filepath.Join(instanceDir, datadirPrivateKey)
	if err := crypto.SaveECDSA(keyfile, key); err != nil {
		log.Error(fmt.Sprintf("Failed to persist node key: %v", err))
	}
	return key
}

//static nodes返回配置为静态节点的节点enode URL的列表。
func (c *Config) StaticNodes() []*enode.Node {
	return c.parsePersistentNodes(&c.staticNodesWarning, c.ResolvePath(datadirStaticNodes))
}

//trusted nodes返回配置为受信任节点的节点enode URL的列表。
func (c *Config) TrustedNodes() []*enode.Node {
	return c.parsePersistentNodes(&c.trustedNodesWarning, c.ResolvePath(datadirTrustedNodes))
}

//ParsePersistentNodes分析从.json加载的发现节点URL列表
//数据目录中的文件。
func (c *Config) parsePersistentNodes(w *bool, path string) []*enode.Node {
//如果不存在节点配置，则短路
	if c.DataDir == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	c.warnOnce(w, "Found deprecated node list file %s, please use the TOML config file instead.", path)

//从配置文件加载节点。
	var nodelist []string
	if err := common.LoadJSON(path, &nodelist); err != nil {
		log.Error(fmt.Sprintf("Can't load node list file: %v", err))
		return nil
	}
//将列表解释为发现节点数组
	var nodes []*enode.Node
	for _, url := range nodelist {
		if url == "" {
			continue
		}
		node, err := enode.ParseV4(url)
		if err != nil {
			log.Error(fmt.Sprintf("Node URL %s: %v\n", url, err))
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes
}

//accountconfig确定scrypt和keydirectory的设置
func (c *Config) AccountConfig() (int, int, string, error) {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	if c.UseLightweightKDF {
		scryptN = keystore.LightScryptN
		scryptP = keystore.LightScryptP
	}

	var (
		keydir string
		err    error
	)
	switch {
	case filepath.IsAbs(c.KeyStoreDir):
		keydir = c.KeyStoreDir
	case c.DataDir != "":
		if c.KeyStoreDir == "" {
			keydir = filepath.Join(c.DataDir, datadirDefaultKeyStore)
		} else {
			keydir, err = filepath.Abs(c.KeyStoreDir)
		}
	case c.KeyStoreDir != "":
		keydir, err = filepath.Abs(c.KeyStoreDir)
	}
	return scryptN, scryptP, keydir, err
}

func makeAccountManager(conf *Config) (*accounts.Manager, string, error) {
	scryptN, scryptP, keydir, err := conf.AccountConfig()
	var ephemeral string
	if keydir == "" {
//没有datadir。
		keydir, err = ioutil.TempDir("", "go-ethereum-keystore")
		ephemeral = keydir
	}

	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(keydir, 0700); err != nil {
		return nil, "", err
	}
//集合客户经理和支持的后端
	backends := []accounts.Backend{
		keystore.NewKeyStore(keydir, scryptN, scryptP),
	}
	if !conf.NoUSB {
//启动用于分类帐硬件钱包的USB集线器
		if ledgerhub, err := usbwallet.NewLedgerHub(); err != nil {
			log.Warn(fmt.Sprintf("Failed to start Ledger hub, disabling: %v", err))
		} else {
			backends = append(backends, ledgerhub)
		}
//启动Trezor硬件钱包的USB集线器
		if trezorhub, err := usbwallet.NewTrezorHub(); err != nil {
			log.Warn(fmt.Sprintf("Failed to start Trezor hub, disabling: %v", err))
		} else {
			backends = append(backends, trezorhub)
		}
	}
	return accounts.NewManager(backends...), ephemeral, nil
}

var warnLock sync.Mutex

func (c *Config) warnOnce(w *bool, format string, args ...interface{}) {
	warnLock.Lock()
	defer warnLock.Unlock()

	if *w {
		return
	}
	l := c.Logger
	if l == nil {
		l = log.Root()
	}
	l.Warn(fmt.Sprintf(format, args...))
	*w = true
}
