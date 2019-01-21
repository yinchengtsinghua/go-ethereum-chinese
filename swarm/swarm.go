
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

package swarm

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"io"
	"math/big"
	"net"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/chequebook"
	"github.com/ethereum/go-ethereum/contracts/ens"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/api"
	httpapi "github.com/ethereum/go-ethereum/swarm/api/http"
	"github.com/ethereum/go-ethereum/swarm/fuse"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/network/stream"
	"github.com/ethereum/go-ethereum/swarm/pss"
	"github.com/ethereum/go-ethereum/swarm/state"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/storage/feed"
	"github.com/ethereum/go-ethereum/swarm/storage/mock"
	"github.com/ethereum/go-ethereum/swarm/swap"
	"github.com/ethereum/go-ethereum/swarm/tracing"
)

var (
	startTime          time.Time
	updateGaugesPeriod = 5 * time.Second
	startCounter       = metrics.NewRegisteredCounter("stack,start", nil)
	stopCounter        = metrics.NewRegisteredCounter("stack,stop", nil)
	uptimeGauge        = metrics.NewRegisteredGauge("stack.uptime", nil)
	requestsCacheGauge = metrics.NewRegisteredGauge("storage.cache.requests.size", nil)
)

//群堆栈
type Swarm struct {
config            *api.Config        //群配置
api               *api.API           //高级API层（fs/manifest）
dns               api.Resolver       //域名服务器注册
fileStore         *storage.FileStore //分布式预映像存档，本地API到具有文档级存储/检索支持的存储
	streamer          *stream.Registry
bzz               *network.Bzz       //物流经理
backend           chequebook.Backend //简单区块链后端
	privateKey        *ecdsa.PrivateKey
	netStore          *storage.NetStore
sfs               *fuse.SwarmFS //需要此操作来清除节点出口上的所有活动装载
	ps                *pss.Pss
	swap              *swap.Swap
	stateStore        *state.DBStore
	accountingMetrics *protocols.AccountingMetrics

	tracerClose io.Closer
}

//创建新的Swarm服务实例
//实现节点服务
//如果mockstore不是nil，它将用作块数据的存储。
//mockstore只能用于测试。
func NewSwarm(config *api.Config, mockStore *mock.NodeStore) (self *Swarm, err error) {

	if bytes.Equal(common.FromHex(config.PublicKey), storage.ZeroAddr) {
		return nil, fmt.Errorf("empty public key")
	}
	if bytes.Equal(common.FromHex(config.BzzKey), storage.ZeroAddr) {
		return nil, fmt.Errorf("empty bzz key")
	}

	var backend chequebook.Backend
	if config.SwapAPI != "" && config.SwapEnabled {
		log.Info("connecting to SWAP API", "url", config.SwapAPI)
		backend, err = ethclient.Dial(config.SwapAPI)
		if err != nil {
			return nil, fmt.Errorf("error connecting to SWAP API %s: %s", config.SwapAPI, err)
		}
	}

	self = &Swarm{
		config:     config,
		backend:    backend,
		privateKey: config.ShiftPrivateKey(),
	}
	log.Debug("Setting up Swarm service components")

	config.HiveParams.Discovery = true

	bzzconfig := &network.BzzConfig{
		NetworkID:   config.NetworkID,
		OverlayAddr: common.FromHex(config.BzzKey),
		HiveParams:  config.HiveParams,
		LightNode:   config.LightNodeEnabled,
	}

	self.stateStore, err = state.NewDBStore(filepath.Join(config.Path, "state-store.db"))
	if err != nil {
		return
	}

//设置高级API
	var resolver *api.MultiResolver
	if len(config.EnsAPIs) > 0 {
		opts := []api.MultiResolverOption{}
		for _, c := range config.EnsAPIs {
			tld, endpoint, addr := parseEnsAPIAddress(c)
			r, err := newEnsClient(endpoint, addr, config, self.privateKey)
			if err != nil {
				return nil, err
			}
			opts = append(opts, api.MultiResolverOptionWithResolver(r, tld))

		}
		resolver = api.NewMultiResolver(opts...)
		self.dns = resolver
	}

	lstore, err := storage.NewLocalStore(config.LocalStoreParams, mockStore)
	if err != nil {
		return nil, err
	}

	self.netStore, err = storage.NewNetStore(lstore, nil)
	if err != nil {
		return nil, err
	}

	to := network.NewKademlia(
		common.FromHex(config.BzzKey),
		network.NewKadParams(),
	)
	delivery := stream.NewDelivery(to, self.netStore)
	self.netStore.NewNetFetcherFunc = network.NewFetcherFactory(delivery.RequestFromPeers, config.DeliverySkipCheck).New

	if config.SwapEnabled {
		balancesStore, err := state.NewDBStore(filepath.Join(config.Path, "balances.db"))
		if err != nil {
			return nil, err
		}
		self.swap = swap.New(balancesStore)
		self.accountingMetrics = protocols.SetupAccountingMetrics(10*time.Second, filepath.Join(config.Path, "metrics.db"))
	}

	var nodeID enode.ID
	if err := nodeID.UnmarshalText([]byte(config.NodeID)); err != nil {
		return nil, err
	}

	syncing := stream.SyncingAutoSubscribe
	if !config.SyncEnabled || config.LightNodeEnabled {
		syncing = stream.SyncingDisabled
	}

	retrieval := stream.RetrievalEnabled
	if config.LightNodeEnabled {
		retrieval = stream.RetrievalClientOnly
	}

	registryOptions := &stream.RegistryOptions{
		SkipCheck:       config.DeliverySkipCheck,
		Syncing:         syncing,
		Retrieval:       retrieval,
		SyncUpdateDelay: config.SyncUpdateDelay,
		MaxPeerServers:  config.MaxStreamPeerServers,
	}
	self.streamer = stream.NewRegistry(nodeID, delivery, self.netStore, self.stateStore, registryOptions, self.swap)

//用于任意长度文档/文件存储的Swarm哈希组合块
	self.fileStore = storage.NewFileStore(self.netStore, self.config.FileStoreParams)

	var feedsHandler *feed.Handler
	fhParams := &feed.HandlerParams{}

	feedsHandler = feed.NewHandler(fhParams)
	feedsHandler.SetStore(self.netStore)

	lstore.Validators = []storage.ChunkValidator{
		storage.NewContentAddressValidator(storage.MakeHashFunc(storage.DefaultHash)),
		feedsHandler,
	}

	err = lstore.Migrate()
	if err != nil {
		return nil, err
	}

	log.Debug("Setup local storage")

	self.bzz = network.NewBzz(bzzconfig, to, self.stateStore, self.streamer.GetSpec(), self.streamer.Run)

//pss=Swarm上的邮政服务（bzz上的devp2p）
	self.ps, err = pss.NewPss(to, config.Pss)
	if err != nil {
		return nil, err
	}
	if pss.IsActiveHandshake {
		pss.SetHandshakeController(self.ps, pss.NewHandshakeParams())
	}

	self.api = api.NewAPI(self.fileStore, self.dns, feedsHandler, self.privateKey)

	self.sfs = fuse.NewSwarmFS(self.api)
	log.Debug("Initialized FUSE filesystem")

	return self, nil
}

//parsensapiaddress根据格式分析字符串
//[tld:][contract addr@]url并返回ensclientconfig结构
//包含端点、合同地址和TLD。
func parseEnsAPIAddress(s string) (tld, endpoint string, addr common.Address) {
	isAllLetterString := func(s string) bool {
		for _, r := range s {
			if !unicode.IsLetter(r) {
				return false
			}
		}
		return true
	}
	endpoint = s
	if i := strings.Index(endpoint, ":"); i > 0 {
if isAllLetterString(endpoint[:i]) && len(endpoint) > i+2 && endpoint[i+1:i+3] != "//“{”
			tld = endpoint[:i]
			endpoint = endpoint[i+1:]
		}
	}
	if i := strings.Index(endpoint, "@"); i > 0 {
		addr = common.HexToAddress(endpoint[:i])
		endpoint = endpoint[i+1:]
	}
	return
}

//ensclient为api.resolveValidator提供功能
type ensClient struct {
	*ens.ENS
	*ethclient.Client
}

//new ens client创建一个新的ens客户端，因为它是
//特定端点上的ENS API。它用作辅助函数
//用于在NewsWarm函数中创建多个冲突解决程序。
func newEnsClient(endpoint string, addr common.Address, config *api.Config, privkey *ecdsa.PrivateKey) (*ensClient, error) {
	log.Info("connecting to ENS API", "url", endpoint)
	client, err := rpc.Dial(endpoint)
	if err != nil {
		return nil, fmt.Errorf("error connecting to ENS API %s: %s", endpoint, err)
	}
	ethClient := ethclient.NewClient(client)

	ensRoot := config.EnsRoot
	if addr != (common.Address{}) {
		ensRoot = addr
	} else {
		a, err := detectEnsAddr(client)
		if err == nil {
			ensRoot = a
		} else {
			log.Warn(fmt.Sprintf("could not determine ENS contract address, using default %s", ensRoot), "err", err)
		}
	}
	transactOpts := bind.NewKeyedTransactor(privkey)
	dns, err := ens.NewENS(transactOpts, ensRoot, ethClient)
	if err != nil {
		return nil, err
	}
	log.Debug(fmt.Sprintf("-> Swarm Domain Name Registrar %v @ address %v", endpoint, ensRoot.Hex()))
	return &ensClient{
		ENS:    dns,
		Client: ethClient,
	}, err
}

//DetectensAddr通过获取
//版本和Genesis散列使用客户端并将它们与
//主网或测试网地址
func detectEnsAddr(client *rpc.Client) (common.Address, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var version string
	if err := client.CallContext(ctx, &version, "net_version"); err != nil {
		return common.Address{}, err
	}

	block, err := ethclient.NewClient(client).BlockByNumber(ctx, big.NewInt(0))
	if err != nil {
		return common.Address{}, err
	}

	switch {

	case version == "1" && block.Hash() == params.MainnetGenesisHash:
		log.Info("using Mainnet ENS contract address", "addr", ens.MainNetAddress)
		return ens.MainNetAddress, nil

	case version == "3" && block.Hash() == params.TestnetGenesisHash:
		log.Info("using Testnet ENS contract address", "addr", ens.TestNetAddress)
		return ens.TestNetAddress, nil

	default:
		return common.Address{}, fmt.Errorf("unknown version and genesis hash: %s %s", version, block.Hash())
	}
}

/*
启动堆栈时调用Start
*启动网络Kademlia Hive对等管理
*（启动NetStore 0级API）
*启动DPA 1级API（分块->存储/检索请求）
*（启动2级API）
*启动HTTP代理服务器
*为BZZ等注册URL方案处理程序
*TODO：开始剑、发誓、蜂群等子服务
**/

//实现node.service接口
func (self *Swarm) Start(srv *p2p.Server) error {
	startTime = time.Now()

	self.tracerClose = tracing.Closer

//更新uaddr以更正enode
	newaddr := self.bzz.UpdateLocalAddr([]byte(srv.Self().String()))
	log.Info("Updated bzz local addr", "oaddr", fmt.Sprintf("%x", newaddr.OAddr), "uaddr", fmt.Sprintf("%s", newaddr.UAddr))
//集支票簿
//TODO:当前如果启用了交换，并且没有提供支票簿（或不存在的）合同，那么节点将崩溃。
//一旦我们把合同整合回去，这项检查必须重新进行。
	if self.config.SwapEnabled && self.config.SwapAPI != "" {
ctx := context.Background() //初始设置没有截止时间。
		err := self.SetChequebook(ctx)
		if err != nil {
			return fmt.Errorf("Unable to set chequebook for SWAP: %v", err)
		}
		log.Debug(fmt.Sprintf("-> cheque book for SWAP: %v", self.config.Swap.Chequebook()))
	} else {
		log.Debug(fmt.Sprintf("SWAP disabled: no cheque book set"))
	}

	log.Info("Starting bzz service")

	err := self.bzz.Start(srv)
	if err != nil {
		log.Error("bzz failed", "err", err)
		return err
	}
	log.Info("Swarm network started", "bzzaddr", fmt.Sprintf("%x", self.bzz.Hive.BaseAddr()))

	if self.ps != nil {
		self.ps.Start(srv)
	}

//启动Swarm HTTP代理服务器
	if self.config.Port != "" {
		addr := net.JoinHostPort(self.config.ListenAddr, self.config.Port)
		server := httpapi.NewServer(self.api, self.config.Cors)

		if self.config.Cors != "" {
			log.Debug("Swarm HTTP proxy CORS headers", "allowedOrigins", self.config.Cors)
		}

		log.Debug("Starting Swarm HTTP proxy", "port", self.config.Port)
		go func() {
			err := server.ListenAndServe(addr)
			if err != nil {
				log.Error("Could not start Swarm HTTP proxy", "err", err.Error())
			}
		}()
	}

	self.periodicallyUpdateGauges()

	startCounter.Inc(1)
	self.streamer.Start(srv)
	return nil
}

func (self *Swarm) periodicallyUpdateGauges() {
	ticker := time.NewTicker(updateGaugesPeriod)

	go func() {
		for range ticker.C {
			self.updateGauges()
		}
	}()
}

func (self *Swarm) updateGauges() {
	uptimeGauge.Update(time.Since(startTime).Nanoseconds())
	requestsCacheGauge.Update(int64(self.netStore.RequestsCacheLen()))
}

//实现node.service接口
//停止所有组件服务。
func (self *Swarm) Stop() error {
	if self.tracerClose != nil {
		err := self.tracerClose.Close()
		if err != nil {
			return err
		}
	}

	if self.ps != nil {
		self.ps.Stop()
	}
	if ch := self.config.Swap.Chequebook(); ch != nil {
		ch.Stop()
		ch.Save()
	}
	if self.swap != nil {
		self.swap.Close()
	}
	if self.accountingMetrics != nil {
		self.accountingMetrics.Close()
	}
	if self.netStore != nil {
		self.netStore.Close()
	}
	self.sfs.Stop()
	stopCounter.Inc(1)
	self.streamer.Stop()

	err := self.bzz.Stop()
	if self.stateStore != nil {
		self.stateStore.Close()
	}
	return err
}

//实现node.service接口
func (self *Swarm) Protocols() (protos []p2p.Protocol) {
	protos = append(protos, self.bzz.Protocols()...)

	if self.ps != nil {
		protos = append(protos, self.ps.Protocols()...)
	}
	return
}

//实现节点服务
//API返回Swarm实现提供的RPC API描述符
func (self *Swarm) APIs() []rpc.API {

	apis := []rpc.API{
//公共API
		{
			Namespace: "bzz",
			Version:   "3.0",
			Service:   &Info{self.config, chequebook.ContractParams},
			Public:    true,
		},
//管理API
		{
			Namespace: "bzz",
			Version:   "3.0",
			Service:   api.NewControl(self.api, self.bzz.Hive),
			Public:    false,
		},
		{
			Namespace: "chequebook",
			Version:   chequebook.Version,
			Service:   chequebook.NewApi(self.config.Swap.Chequebook),
			Public:    false,
		},
		{
			Namespace: "swarmfs",
			Version:   fuse.Swarmfs_Version,
			Service:   self.sfs,
			Public:    false,
		},
		{
			Namespace: "accounting",
			Version:   protocols.AccountingVersion,
			Service:   protocols.NewAccountingApi(self.accountingMetrics),
			Public:    false,
		},
	}

	apis = append(apis, self.bzz.APIs()...)

	if self.ps != nil {
		apis = append(apis, self.ps.APIs()...)
	}

	return apis
}

//setcheckbook确保本地checquebook建立在链上。
func (self *Swarm) SetChequebook(ctx context.Context) error {
	err := self.config.Swap.SetChequebook(ctx, self.backend, self.config.Path)
	if err != nil {
		return err
	}
	log.Info(fmt.Sprintf("new chequebook set (%v): saving config file, resetting all connections in the hive", self.config.Swap.Contract.Hex()))
	return nil
}

//关于Swarm的可序列化信息
type Info struct {
	*api.Config
	*chequebook.Params
}

func (self *Info) Info() *Info {
	return self
}
