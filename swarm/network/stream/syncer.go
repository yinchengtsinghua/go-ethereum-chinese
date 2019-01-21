
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

package stream

import (
	"context"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

const (
	BatchSize = 128
)

//swarmsyncerserver实现在存储箱上同步历史记录的服务器
//提供的流：
//*带或不带支票的实时请求交付
//（实时/非实时历史记录）每个邻近箱的块同步
type SwarmSyncerServer struct {
	po    uint8
	store storage.SyncChunkStore
	quit  chan struct{}
}

//newswarmsyncerserver是swarmsyncerserver的构造函数
func NewSwarmSyncerServer(po uint8, syncChunkStore storage.SyncChunkStore) (*SwarmSyncerServer, error) {
	return &SwarmSyncerServer{
		po:    po,
		store: syncChunkStore,
		quit:  make(chan struct{}),
	}, nil
}

func RegisterSwarmSyncerServer(streamer *Registry, syncChunkStore storage.SyncChunkStore) {
	streamer.RegisterServerFunc("SYNC", func(_ *Peer, t string, _ bool) (Server, error) {
		po, err := ParseSyncBinKey(t)
		if err != nil {
			return nil, err
		}
		return NewSwarmSyncerServer(po, syncChunkStore)
	})
//streamer.registerserverfunc（stream，func（p*peer）（服务器，错误）
//返回newoutgoingprovableswarmsyncer（po，db）
//}）
}

//需要在流服务器上调用Close
func (s *SwarmSyncerServer) Close() {
	close(s.quit)
}

//getdata从netstore检索实际块
func (s *SwarmSyncerServer) GetData(ctx context.Context, key []byte) ([]byte, error) {
	chunk, err := s.store.Get(ctx, storage.Address(key))
	if err != nil {
		return nil, err
	}
	return chunk.Data(), nil
}

//sessionindex返回当前存储箱（po）索引。
func (s *SwarmSyncerServer) SessionIndex() (uint64, error) {
	return s.store.BinIndex(s.po), nil
}

//getbatch从dbstore检索下一批哈希
func (s *SwarmSyncerServer) SetNextBatch(from, to uint64) ([]byte, uint64, uint64, *HandoverProof, error) {
	var batch []byte
	i := 0

	var ticker *time.Ticker
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
	}()
	var wait bool
	for {
		if wait {
			if ticker == nil {
				ticker = time.NewTicker(1000 * time.Millisecond)
			}
			select {
			case <-ticker.C:
			case <-s.quit:
				return nil, 0, 0, nil, nil
			}
		}

		metrics.GetOrRegisterCounter("syncer.setnextbatch.iterator", nil).Inc(1)
		err := s.store.Iterator(from, to, s.po, func(key storage.Address, idx uint64) bool {
			batch = append(batch, key[:]...)
			i++
			to = idx
			return i < BatchSize
		})
		if err != nil {
			return nil, 0, 0, nil, err
		}
		if len(batch) > 0 {
			break
		}
		wait = true
	}

	log.Trace("Swarm syncer offer batch", "po", s.po, "len", i, "from", from, "to", to, "current store count", s.store.BinIndex(s.po))
	return batch, from, to, nil, nil
}

//垃圾同步机
type SwarmSyncerClient struct {
	store  storage.SyncChunkStore
	peer   *Peer
	stream Stream
}

//NewsWarmSyncerClient是可验证数据交换同步器的控制器
func NewSwarmSyncerClient(p *Peer, store storage.SyncChunkStore, stream Stream) (*SwarmSyncerClient, error) {
	return &SwarmSyncerClient{
		store:  store,
		peer:   p,
		stream: stream,
	}, nil
}

////newincomingprovableswamsyncer是可验证数据交换同步器的控制器
//func newincomingprovableswarmsyncer（po int，priority int，index uint64，sessionna uint64，interval[]uint64，sessionroot storage.address，chunker*storage.pyramidchunker，store storage.chunkstore，p peer）*swarmsyncerclient
//检索：=make（storage.chunk，chunkscap）
//runchunkrequester（P，检索）
//storec:=make（storage.chunk，chunkscap）
//runchunkstorer（商店、商店）
//S：=和SwarmSyncerClient
//采购订单：采购订单，
//优先级：优先级，
//sessiona:会话，
//开始：索引，
//结束：索引，
//下一步：制造（Chan结构，1）
//间隔：间隔，
//sessionroot:会话根，
//sessionreader:chunker.join（sessionroot，retrievec），
//检索：检索，
//storec：storec，
//}
//返回S
//}

////在对等机上调用StartSyncing以启动同步进程
////其理念是只有当卡德米利亚接近健康时才调用它
//func开始同步（s*拖缆，peerid enode.id，po uint8，nn bool）
//拉斯坡
//如果神经网络{
//LaSTPO＝Max
//}
//
//对于i：=po；i<=lastpo；i++
//s.subscribe（peerid，“同步”，newsynclabel（“实时”，po），0，0，high，true）
//s.subscribe（peerid，“同步”，newsynclabel（“历史”，po），0，0，mid，false）
//}
//}

//registerwarmsyncerclient为注册客户端构造函数函数
//处理传入的同步流
func RegisterSwarmSyncerClient(streamer *Registry, store storage.SyncChunkStore) {
	streamer.RegisterClientFunc("SYNC", func(p *Peer, t string, live bool) (Client, error) {
		return NewSwarmSyncerClient(p, store, NewStream("SYNC", t, live))
	})
}

//需求数据
func (s *SwarmSyncerClient) NeedData(ctx context.Context, key []byte) (wait func(context.Context) error) {
	return s.store.FetchFunc(ctx, key)
}

//巴奇多
func (s *SwarmSyncerClient) BatchDone(stream Stream, from uint64, hashes []byte, root []byte) func() (*TakeoverProof, error) {
//TODO:使用putter/getter重构代码重新启用此项
//如果S.Cukes！= nIL{
//return func（）（*takeoveroof，error）返回s.takeoveroof（stream，from，hashes，root）
//}
	return nil
}

func (s *SwarmSyncerClient) Close() {}

//分析和格式化同步bin键的基础
//它必须是2<=基<=36
const syncBinKeyBase = 36

//FormatSyncBinkey返回的字符串表示形式
//要用作同步流密钥的Kademlia bin号。
func FormatSyncBinKey(bin uint8) string {
	return strconv.FormatUint(uint64(bin), syncBinKeyBase)
}

//ParseSyncBinKey分析字符串表示形式
//并返回Kademlia bin编号。
func ParseSyncBinKey(s string) (uint8, error) {
	bin, err := strconv.ParseUint(s, syncBinKeyBase, 8)
	if err != nil {
		return 0, err
	}
	return uint8(bin), nil
}
