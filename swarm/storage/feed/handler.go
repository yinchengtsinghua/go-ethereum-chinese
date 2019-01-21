
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

//处理程序是源的API
//它支持创建、更新、同步和检索源更新及其数据
package feed

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/swarm/storage/feed/lookup"

	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

type Handler struct {
	chunkStore *storage.NetStore
	HashSize   int
	cache      map[uint64]*cacheEntry
	cacheLock  sync.RWMutex
}

//handlerParams将参数传递给处理程序构造函数newhandler
//签名者和时间戳提供程序是必需的参数
type HandlerParams struct {
}

//哈希池包含一个准备好的哈希器池
var hashPool sync.Pool

//init初始化包和哈希池
func init() {
	hashPool = sync.Pool{
		New: func() interface{} {
			return storage.MakeHashFunc(feedsHashAlgorithm)()
		},
	}
}

//NewHandler创建了一个新的Swarm Feeds API
func NewHandler(params *HandlerParams) *Handler {
	fh := &Handler{
		cache: make(map[uint64]*cacheEntry),
	}

	for i := 0; i < hasherCount; i++ {
		hashfunc := storage.MakeHashFunc(feedsHashAlgorithm)()
		if fh.HashSize == 0 {
			fh.HashSize = hashfunc.Size()
		}
		hashPool.Put(hashfunc)
	}

	return fh
}

//setstore为swarm feeds api设置存储后端
func (h *Handler) SetStore(store *storage.NetStore) {
	h.chunkStore = store
}

//validate是块验证方法
//如果它看起来像一个提要更新，那么块地址将根据更新签名的useraddr进行检查。
//它实现了storage.chunkvalidator接口
func (h *Handler) Validate(chunk storage.Chunk) bool {
	if len(chunk.Data()) < minimumSignedUpdateLength {
		return false
	}

//检查它是否是格式正确的更新块
//所尝试的源的有效签名和所有权证明
//更新

//首先，反序列化块
	var r Request
	if err := r.fromChunk(chunk); err != nil {
		log.Debug("Invalid feed update chunk", "addr", chunk.Address(), "err", err)
		return false
	}

//验证签名，并且签名者实际上拥有源
//如果失败，则表示签名无效，数据已损坏。
//或者有人试图更新其他人的订阅源。
	if err := r.Verify(); err != nil {
		log.Debug("Invalid feed update signature", "err", err)
		return false
	}

	return true
}

//getContent检索源的上次同步更新的数据负载
func (h *Handler) GetContent(feed *Feed) (storage.Address, []byte, error) {
	if feed == nil {
		return nil, nil, NewError(ErrInvalidValue, "feed is nil")
	}
	feedUpdate := h.get(feed)
	if feedUpdate == nil {
		return nil, nil, NewError(ErrNotFound, "feed update not cached")
	}
	return feedUpdate.lastKey, feedUpdate.data, nil
}

//NewRequest准备一个请求结构，其中包含
//只需添加所需数据并签名即可。
//然后可以对生成的结构进行签名并将其传递给handler.update以进行验证并发送
func (h *Handler) NewRequest(ctx context.Context, feed *Feed) (request *Request, err error) {
	if feed == nil {
		return nil, NewError(ErrInvalidValue, "feed cannot be nil")
	}

	now := TimestampProvider.Now().Time
	request = new(Request)
	request.Header.Version = ProtocolVersion

	query := NewQueryLatest(feed, lookup.NoClue)

	feedUpdate, err := h.Lookup(ctx, query)
	if err != nil {
		if err.(*Error).code != ErrNotFound {
			return nil, err
		}
//找不到更新意味着存在网络错误
//或者订阅源确实没有更新
	}

	request.Feed = *feed

//如果我们已经有了更新，那么找到下一个时代
	if feedUpdate != nil {
		request.Epoch = lookup.GetNextEpoch(feedUpdate.Epoch, now)
	} else {
		request.Epoch = lookup.GetFirstEpoch(now)
	}

	return request, nil
}

//查找检索特定或最新的源更新
//根据“query”的配置，查找的工作方式不同
//请参阅“query”文档和帮助器函数：
//`newquerylatest`和'newquery`
func (h *Handler) Lookup(ctx context.Context, query *Query) (*cacheEntry, error) {

	timeLimit := query.TimeLimit
if timeLimit == 0 { //如果时间限制设置为零，则用户希望获取最新更新
		timeLimit = TimestampProvider.Now().Time
	}

if query.Hint == lookup.NoClue { //尝试使用我们的缓存
		entry := h.get(&query.Feed)
if entry != nil && entry.Epoch.Time <= timeLimit { //避免不良提示
			query.Hint = entry.Epoch
		}
	}

//没有商店我们找不到任何东西
	if h.chunkStore == nil {
		return nil, NewError(ErrInit, "Call Handler.SetStore() before performing lookups")
	}

	var id ID
	id.Feed = query.Feed
	var readCount int

//调用查找引擎。
//每次查找算法需要猜测时都将调用回调
	requestPtr, err := lookup.Lookup(timeLimit, query.Hint, func(epoch lookup.Epoch, now uint64) (interface{}, error) {
		readCount++
		id.Epoch = epoch
		ctx, cancel := context.WithTimeout(ctx, defaultRetrieveTimeout)
		defer cancel()

		chunk, err := h.chunkStore.Get(ctx, id.Addr())
if err != nil { //TODO:未找到块以外的灾难性错误
			return nil, nil
		}

		var request Request
		if err := request.fromChunk(chunk); err != nil {
			return nil, nil
		}
		if request.Time <= timeLimit {
			return &request, nil
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}

	log.Info(fmt.Sprintf("Feed lookup finished in %d lookups", readCount))

	request, _ := requestPtr.(*Request)
	if request == nil {
		return nil, NewError(ErrNotFound, "no feed updates found")
	}
	return h.updateCache(request)

}

//使用指定内容更新源更新缓存
func (h *Handler) updateCache(request *Request) (*cacheEntry, error) {

	updateAddr := request.Addr()
	log.Trace("feed cache update", "topic", request.Topic.Hex(), "updateaddr", updateAddr, "epoch time", request.Epoch.Time, "epoch level", request.Epoch.Level)

	feedUpdate := h.get(&request.Feed)
	if feedUpdate == nil {
		feedUpdate = &cacheEntry{}
		h.set(&request.Feed, feedUpdate)
	}

//更新我们的RSRCS入口地图
	feedUpdate.lastKey = updateAddr
	feedUpdate.Update = request.Update
	feedUpdate.Reader = bytes.NewReader(feedUpdate.data)
	return feedUpdate, nil
}

//更新发布源更新
//请注意，提要更新不能跨越块，因此具有最大净长度4096，包括更新头数据和签名。
//这将导致最大负载为“maxupdatedatalength”（有关详细信息，请检查update.go）
//如果区块负载的总长度将超过此限制，则返回错误。
//更新只能检查调用方是否试图覆盖最新的已知版本，否则它只会将更新
//在网络上。
func (h *Handler) Update(ctx context.Context, r *Request) (updateAddr storage.Address, err error) {

//没有商店我们无法更新任何内容
	if h.chunkStore == nil {
		return nil, NewError(ErrInit, "Call Handler.SetStore() before updating")
	}

	feedUpdate := h.get(&r.Feed)
if feedUpdate != nil && feedUpdate.Epoch.Equals(r.Epoch) { //这是我们唯一能确定的便宜支票
		return nil, NewError(ErrInvalidValue, "A former update in this epoch is already known to exist")
	}

chunk, err := r.toChunk() //将更新序列化为块。如果数据太大则失败
	if err != nil {
		return nil, err
	}

//发送块
	h.chunkStore.Put(ctx, chunk)
	log.Trace("feed update", "updateAddr", r.idAddr, "epoch time", r.Epoch.Time, "epoch level", r.Epoch.Level, "data", chunk.Data())
//更新我们的feed更新映射缓存条目，如果新的更新比我们现有的更新旧，如果我们有。
	if feedUpdate != nil && r.Epoch.After(feedUpdate.Epoch) {
		feedUpdate.Epoch = r.Epoch
		feedUpdate.data = make([]byte, len(r.data))
		feedUpdate.lastKey = r.idAddr
		copy(feedUpdate.data, r.data)
		feedUpdate.Reader = bytes.NewReader(feedUpdate.data)
	}

	return r.idAddr, nil
}

//检索给定名称哈希的源更新缓存值
func (h *Handler) get(feed *Feed) *cacheEntry {
	mapKey := feed.mapKey()
	h.cacheLock.RLock()
	defer h.cacheLock.RUnlock()
	feedUpdate := h.cache[mapKey]
	return feedUpdate
}

//设置给定源的源更新缓存值
func (h *Handler) set(feed *Feed, feedUpdate *cacheEntry) {
	mapKey := feed.mapKey()
	h.cacheLock.Lock()
	defer h.cacheLock.Unlock()
	h.cache[mapKey] = feedUpdate
}
