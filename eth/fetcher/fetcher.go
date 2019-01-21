
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

//包获取器包含基于块通知的同步。
package fetcher

import (
	"errors"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/prque"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

const (
arriveTimeout = 500 * time.Millisecond //明确请求公告块之前的时间裕度
gatherSlack   = 100 * time.Millisecond //用于整理带有回迁的几乎过期的公告的间隔
fetchTimeout  = 5 * time.Second        //返回显式请求块的最长分配时间
maxUncleDist  = 7                      //距链头的最大允许后退距离
maxQueueDist  = 32                     //链头到队列的最大允许距离
hashLimit     = 256                    //对等机可能已宣布的唯一块的最大数目
blockLimit    = 64                     //对等端传递的唯一块的最大数目
)

var (
	errTerminated = errors.New("terminated")
)

//blockretrievalfn是用于从本地链检索块的回调类型。
type blockRetrievalFn func(common.Hash) *types.Block

//HeaderRequesterFn是用于发送头检索请求的回调类型。
type headerRequesterFn func(common.Hash) error

//BodyRequesterFn是用于发送正文检索请求的回调类型。
type bodyRequesterFn func([]common.Hash) error

//headerverifierfn是一种回调类型，用于验证块头的快速传播。
type headerVerifierFn func(header *types.Header) error

//BlockBroadcasterFn是一种回调类型，用于向连接的对等端广播块。
type blockBroadcasterFn func(block *types.Block, propagate bool)

//chainheightfn是用于检索当前链高度的回调类型。
type chainHeightFn func() uint64

//chaininsertfn是一种回调类型，用于将一批块插入本地链。
type chainInsertFn func(types.Blocks) (int, error)

//peerDropFn是一种回调类型，用于删除被检测为恶意的对等机。
type peerDropFn func(id string)

//Announce是哈希通知，通知中新块的可用性
//网络。
type announce struct {
hash   common.Hash   //正在公布的块的哈希值
number uint64        //正在公布的块的数目（0=未知旧协议）
header *types.Header //部分重新组装的块头（新协议）
time   time.Time     //公告的时间戳

origin string //发出通知的对等方的标识符

fetchHeader headerRequesterFn //获取函数以检索已公告块的头
fetchBodies bodyRequesterFn   //获取函数以检索已公告块的主体
}

//HeaderFilterTask表示需要获取器筛选的一批头。
type headerFilterTask struct {
peer    string          //块头的源对等
headers []*types.Header //要筛选的标题集合
time    time.Time       //收割台到达时间
}

//BodyFilterTask表示一批块体（事务和叔叔）
//需要回迁过滤。
type bodyFilterTask struct {
peer         string                 //阻塞体的源对等体
transactions [][]*types.Transaction //Collection of transactions per block bodies
uncles       [][]*types.Header      //每个区块主体的叔叔集合
time         time.Time              //块内容物到达时间
}

//Inject表示计划导入操作。
type inject struct {
	origin string
	block  *types.Block
}

//Fetcher负责从不同的对等方收集块通知
//并安排它们进行检索。
type Fetcher struct {
//各种事件通道
	notify chan *announce
	inject chan *inject

	blockFilter  chan chan []*types.Block
	headerFilter chan chan *headerFilterTask
	bodyFilter   chan chan *bodyFilterTask

	done chan common.Hash
	quit chan struct{}

//宣布国
announces  map[string]int              //每对等端宣布计数以防止内存耗尽
announced  map[common.Hash][]*announce //已通知块，计划获取
fetching   map[common.Hash]*announce   //Announced blocks, currently fetching
fetched    map[common.Hash][]*announce //已提取头的块，计划用于正文检索
completing map[common.Hash]*announce   //带有标题的块，当前主体正在完成

//块缓存
queue  *prque.Prque            //包含导入操作的队列（已排序块号）
queues map[string]int          //每对等块计数以防止内存耗尽
queued map[common.Hash]*inject //已排队的块集（用于消除导入的重复数据）

//回调
getBlock       blockRetrievalFn   //从本地链中检索块
verifyHeader   headerVerifierFn   //检查块的头是否具有有效的工作证明
broadcastBlock blockBroadcasterFn //向连接的对等端广播块
chainHeight    chainHeightFn      //检索当前链的高度
insertChain    chainInsertFn      //向链中注入一批块
dropPeer       peerDropFn         //因行为不端而丢掉一个同伴

//测试钩
announceChangeHook func(common.Hash, bool) //方法在从公告列表中添加或删除哈希时调用
queueChangeHook    func(common.Hash, bool) //从导入队列添加或删除块时要调用的方法
fetchingHook       func([]common.Hash)     //启动块（eth/61）或头（eth/62）提取时调用的方法
completingHook     func([]common.Hash)     //启动块体提取时调用的方法（eth/62）
importedHook       func(*types.Block)      //成功块导入时调用的方法（ETH/61和ETH/62）
}

//新建创建一个块获取器，以基于哈希通知检索块。
func New(getBlock blockRetrievalFn, verifyHeader headerVerifierFn, broadcastBlock blockBroadcasterFn, chainHeight chainHeightFn, insertChain chainInsertFn, dropPeer peerDropFn) *Fetcher {
	return &Fetcher{
		notify:         make(chan *announce),
		inject:         make(chan *inject),
		blockFilter:    make(chan chan []*types.Block),
		headerFilter:   make(chan chan *headerFilterTask),
		bodyFilter:     make(chan chan *bodyFilterTask),
		done:           make(chan common.Hash),
		quit:           make(chan struct{}),
		announces:      make(map[string]int),
		announced:      make(map[common.Hash][]*announce),
		fetching:       make(map[common.Hash]*announce),
		fetched:        make(map[common.Hash][]*announce),
		completing:     make(map[common.Hash]*announce),
		queue:          prque.New(nil),
		queues:         make(map[string]int),
		queued:         make(map[common.Hash]*inject),
		getBlock:       getBlock,
		verifyHeader:   verifyHeader,
		broadcastBlock: broadcastBlock,
		chainHeight:    chainHeight,
		insertChain:    insertChain,
		dropPeer:       dropPeer,
	}
}

//启动基于公告的同步器，接受和处理
//哈希通知和块提取，直到请求终止。
func (f *Fetcher) Start() {
	go f.loop()
}

//停止终止基于公告的同步器，取消所有挂起
//操作。
func (f *Fetcher) Stop() {
	close(f.quit)
}

//通知将通知获取程序新块的潜在可用性
//网络。
func (f *Fetcher) Notify(peer string, hash common.Hash, number uint64, time time.Time,
	headerFetcher headerRequesterFn, bodyFetcher bodyRequesterFn) error {
	block := &announce{
		hash:        hash,
		number:      number,
		time:        time,
		origin:      peer,
		fetchHeader: headerFetcher,
		fetchBodies: bodyFetcher,
	}
	select {
	case f.notify <- block:
		return nil
	case <-f.quit:
		return errTerminated
	}
}

//入队试图填补取款人未来导入队列的空白。
func (f *Fetcher) Enqueue(peer string, block *types.Block) error {
	op := &inject{
		origin: peer,
		block:  block,
	}
	select {
	case f.inject <- op:
		return nil
	case <-f.quit:
		return errTerminated
	}
}

//filterheaders提取提取提取程序显式请求的所有头，
//退回那些应该以不同方式处理的。
func (f *Fetcher) FilterHeaders(peer string, headers []*types.Header, time time.Time) []*types.Header {
	log.Trace("Filtering headers", "peer", peer, "headers", len(headers))

//将过滤通道发送到获取器
	filter := make(chan *headerFilterTask)

	select {
	case f.headerFilter <- filter:
	case <-f.quit:
		return nil
	}
//请求筛选头列表
	select {
	case filter <- &headerFilterTask{peer: peer, headers: headers, time: time}:
	case <-f.quit:
		return nil
	}
//检索筛选后剩余的邮件头
	select {
	case task := <-filter:
		return task.headers
	case <-f.quit:
		return nil
	}
}

//filterbody提取由
//回迁者，返回那些应该以不同方式处理的。
func (f *Fetcher) FilterBodies(peer string, transactions [][]*types.Transaction, uncles [][]*types.Header, time time.Time) ([][]*types.Transaction, [][]*types.Header) {
	log.Trace("Filtering bodies", "peer", peer, "txs", len(transactions), "uncles", len(uncles))

//将过滤通道发送到获取器
	filter := make(chan *bodyFilterTask)

	select {
	case f.bodyFilter <- filter:
	case <-f.quit:
		return nil, nil
	}
//请求对身体清单的过滤
	select {
	case filter <- &bodyFilterTask{peer: peer, transactions: transactions, uncles: uncles, time: time}:
	case <-f.quit:
		return nil, nil
	}
//检索过滤后剩余的主体
	select {
	case task := <-filter:
		return task.transactions, task.uncles
	case <-f.quit:
		return nil, nil
	}
}

//循环是主获取循环，检查和处理各种通知
//事件。
func (f *Fetcher) loop() {
//迭代块提取，直到请求退出
	fetchTimer := time.NewTimer(0)
	completeTimer := time.NewTimer(0)

	for {
//清除所有过期的块提取
		for hash, announce := range f.fetching {
			if time.Since(announce.time) > fetchTimeout {
				f.forgetHash(hash)
			}
		}
//Import any queued blocks that could potentially fit
		height := f.chainHeight()
		for !f.queue.Empty() {
			op := f.queue.PopItem().(*inject)
			hash := op.block.Hash()
			if f.queueChangeHook != nil {
				f.queueChangeHook(hash, false)
			}
//如果链条或相位过高，请稍后继续
			number := op.block.NumberU64()
			if number > height+1 {
				f.queue.Push(op, -int64(number))
				if f.queueChangeHook != nil {
					f.queueChangeHook(hash, true)
				}
				break
			}
//否则，如果是新鲜的，还是未知的，请尝试导入
			if number+maxUncleDist < height || f.getBlock(hash) != nil {
				f.forgetBlock(hash)
				continue
			}
			f.insert(op.origin, op.block)
		}
//等待外部事件发生
		select {
		case <-f.quit:
//取数器终止，中止所有操作
			return

		case notification := <-f.notify:
//宣布封锁，确保同伴没有给我们注射药物。
			propAnnounceInMeter.Mark(1)

			count := f.announces[notification.origin] + 1
			if count > hashLimit {
				log.Debug("Peer exceeded outstanding announces", "peer", notification.origin, "limit", hashLimit)
				propAnnounceDOSMeter.Mark(1)
				break
			}
//如果我们有一个有效的块号，检查它是否有潜在的用处
			if notification.number > 0 {
				if dist := int64(notification.number) - int64(f.chainHeight()); dist < -maxUncleDist || dist > maxQueueDist {
					log.Debug("Peer discarded announcement", "peer", notification.origin, "number", notification.number, "hash", notification.hash, "distance", dist)
					propAnnounceDropMeter.Mark(1)
					break
				}
			}
//一切都很好，如果块尚未下载，请安排公告
			if _, ok := f.fetching[notification.hash]; ok {
				break
			}
			if _, ok := f.completing[notification.hash]; ok {
				break
			}
			f.announces[notification.origin] = count
			f.announced[notification.hash] = append(f.announced[notification.hash], notification)
			if f.announceChangeHook != nil && len(f.announced[notification.hash]) == 1 {
				f.announceChangeHook(notification.hash, true)
			}
			if len(f.announced) == 1 {
				f.rescheduleFetch(fetchTimer)
			}

		case op := <-f.inject:
//已请求直接插入块，请尝试填充所有挂起的空白。
			propBroadcastInMeter.Mark(1)
			f.enqueue(op.origin, op.block)

		case hash := <-f.done:
//挂起的导入已完成，请删除通知的所有跟踪
			f.forgetHash(hash)
			f.forgetBlock(hash)

		case <-fetchTimer.C:
//At least one block's timer ran out, check for needing retrieval
			request := make(map[string][]common.Hash)

			for hash, announces := range f.announced {
				if time.Since(announces[0].time) > arriveTimeout-gatherSlack {
//选择要检索的随机对等机，重置所有其他对等机
					announce := announces[rand.Intn(len(announces))]
					f.forgetHash(hash)

//如果块仍未到达，请排队取件
					if f.getBlock(hash) == nil {
						request[announce.origin] = append(request[announce.origin], hash)
						f.fetching[hash] = announce
					}
				}
			}
//Send out all block header requests
			for peer, hashes := range request {
				log.Trace("Fetching scheduled headers", "peer", peer, "list", hashes)

//在新线程上创建fetch和schedule的闭包
				fetchHeader, hashes := f.fetching[hashes[0]].fetchHeader, hashes
				go func() {
					if f.fetchingHook != nil {
						f.fetchingHook(hashes)
					}
					for _, hash := range hashes {
						headerFetchMeter.Mark(1)
fetchHeader(hash) //次优，但协议不允许批头检索
					}
				}()
			}
//如果块仍处于挂起状态，则计划下一次提取
			f.rescheduleFetch(fetchTimer)

		case <-completeTimer.C:
//至少有一个头的计时器用完了，检索所有内容
			request := make(map[string][]common.Hash)

			for hash, announces := range f.fetched {
//选择要检索的随机对等机，重置所有其他对等机
				announce := announces[rand.Intn(len(announces))]
				f.forgetHash(hash)

//如果块仍未到达，请排队等待完成
				if f.getBlock(hash) == nil {
					request[announce.origin] = append(request[announce.origin], hash)
					f.completing[hash] = announce
				}
			}
//发送所有块体请求
			for peer, hashes := range request {
				log.Trace("Fetching scheduled bodies", "peer", peer, "list", hashes)

//在新线程上创建fetch和schedule的闭包
				if f.completingHook != nil {
					f.completingHook(hashes)
				}
				bodyFetchMeter.Mark(int64(len(hashes)))
				go f.completing[hashes[0]].fetchBodies(hashes)
			}
//如果块仍处于挂起状态，则计划下一次提取
			f.rescheduleComplete(completeTimer)

		case filter := <-f.headerFilter:
//Headers arrived from a remote peer. Extract those that were explicitly
//由提取者请求，并返回所有其他内容，以便交付
//系统的其他部分。
			var task *headerFilterTask
			select {
			case task = <-filter:
			case <-f.quit:
				return
			}
			headerFilterInMeter.Mark(int64(len(task.headers)))

//将一批报头拆分为未知的报文（返回给呼叫者），
//已知的不完整块（需要检索主体）和完整块。
			unknown, incomplete, complete := []*types.Header{}, []*announce{}, []*types.Block{}
			for _, header := range task.headers {
				hash := header.Hash()

//过滤器从其他同步算法中获取请求的头
				if announce := f.fetching[hash]; announce != nil && announce.origin == task.peer && f.fetched[hash] == nil && f.completing[hash] == nil && f.queued[hash] == nil {
//如果交付的报头与承诺的号码不匹配，请删除播音员
					if header.Number.Uint64() != announce.number {
						log.Trace("Invalid block number fetched", "peer", announce.origin, "hash", header.Hash(), "announced", announce.number, "provided", header.Number)
						f.dropPeer(announce.origin)
						f.forgetHash(hash)
						continue
					}
//仅在不通过其他方式进口时保留
					if f.getBlock(hash) == nil {
						announce.header = header
						announce.time = task.time

//如果块为空（仅限头段），则对最终导入队列短路
						if header.TxHash == types.DeriveSha(types.Transactions{}) && header.UncleHash == types.CalcUncleHash([]*types.Header{}) {
							log.Trace("Block empty, skipping body retrieval", "peer", announce.origin, "number", header.Number, "hash", header.Hash())

							block := types.NewBlockWithHeader(header)
							block.ReceivedAt = task.time

							complete = append(complete, block)
							f.completing[hash] = announce
							continue
						}
//否则添加到需要完成的块列表中
						incomplete = append(incomplete, announce)
					} else {
						log.Trace("Block already imported, discarding header", "peer", announce.origin, "number", header.Number, "hash", header.Hash())
						f.forgetHash(hash)
					}
				} else {
//Fetcher不知道这一点，添加到返回列表
					unknown = append(unknown, header)
				}
			}
			headerFilterOutMeter.Mark(int64(len(unknown)))
			select {
			case filter <- &headerFilterTask{headers: unknown, time: task.time}:
			case <-f.quit:
				return
			}
//安排检索到的邮件头的正文完成时间
			for _, announce := range incomplete {
				hash := announce.header.Hash()
				if _, ok := f.completing[hash]; ok {
					continue
				}
				f.fetched[hash] = append(f.fetched[hash], announce)
				if len(f.fetched) == 1 {
					f.rescheduleComplete(completeTimer)
				}
			}
//为导入计划仅标题块
			for _, block := range complete {
				if announce := f.completing[block.Hash()]; announce != nil {
					f.enqueue(announce.origin, block)
				}
			}

		case filter := <-f.bodyFilter:
//块体到达，提取任何显式请求的块，返回其余的
			var task *bodyFilterTask
			select {
			case task = <-filter:
			case <-f.quit:
				return
			}
			bodyFilterInMeter.Mark(int64(len(task.transactions)))

			blocks := []*types.Block{}
			for i := 0; i < len(task.transactions) && i < len(task.uncles); i++ {
//将主体与任何可能的完成请求匹配
				matched := false

				for hash, announce := range f.completing {
					if f.queued[hash] == nil {
						txnHash := types.DeriveSha(types.Transactions(task.transactions[i]))
						uncleHash := types.CalcUncleHash(task.uncles[i])

						if txnHash == announce.header.TxHash && uncleHash == announce.header.UncleHash && announce.origin == task.peer {
//标记匹配的车身，如果仍然未知，则重新装配
							matched = true

							if f.getBlock(hash) == nil {
								block := types.NewBlockWithHeader(announce.header).WithBody(task.transactions[i], task.uncles[i])
								block.ReceivedAt = task.time

								blocks = append(blocks, block)
							} else {
								f.forgetHash(hash)
							}
						}
					}
				}
				if matched {
					task.transactions = append(task.transactions[:i], task.transactions[i+1:]...)
					task.uncles = append(task.uncles[:i], task.uncles[i+1:]...)
					i--
					continue
				}
			}

			bodyFilterOutMeter.Mark(int64(len(task.transactions)))
			select {
			case filter <- task:
			case <-f.quit:
				return
			}
//为有序导入计划检索的块
			for _, block := range blocks {
				if announce := f.completing[block.Hash()]; announce != nil {
					f.enqueue(announce.origin, block)
				}
			}
		}
	}
}

//RescheduleFetch将指定的Fetch计时器重置为下一个公告超时。
func (f *Fetcher) rescheduleFetch(fetch *time.Timer) {
//如果没有公告块，则短路
	if len(f.announced) == 0 {
		return
	}
//否则查找最早的过期通知
	earliest := time.Now()
	for _, announces := range f.announced {
		if earliest.After(announces[0].time) {
			earliest = announces[0].time
		}
	}
	fetch.Reset(arriveTimeout - time.Since(earliest))
}

//重新安排完成将指定的完成计时器重置为下一个提取超时。
func (f *Fetcher) rescheduleComplete(complete *time.Timer) {
//如果未提取收割台，则短路
	if len(f.fetched) == 0 {
		return
	}
//否则查找最早的过期通知
	earliest := time.Now()
	for _, announces := range f.fetched {
		if earliest.After(announces[0].time) {
			earliest = announces[0].time
		}
	}
	complete.Reset(gatherSlack - time.Since(earliest))
}

//如果要导入的块
//还没有看到。
func (f *Fetcher) enqueue(peer string, block *types.Block) {
	hash := block.Hash()

//确保同伴没有给我们剂量
	count := f.queues[peer] + 1
	if count > blockLimit {
		log.Debug("Discarded propagated block, exceeded allowance", "peer", peer, "number", block.Number(), "hash", hash, "limit", blockLimit)
		propBroadcastDOSMeter.Mark(1)
		f.forgetHash(hash)
		return
	}
//丢弃任何过去或太远的块
	if dist := int64(block.NumberU64()) - int64(f.chainHeight()); dist < -maxUncleDist || dist > maxQueueDist {
		log.Debug("Discarded propagated block, too far away", "peer", peer, "number", block.Number(), "hash", hash, "distance", dist)
		propBroadcastDropMeter.Mark(1)
		f.forgetHash(hash)
		return
	}
//为以后的导入计划块
	if _, ok := f.queued[hash]; !ok {
		op := &inject{
			origin: peer,
			block:  block,
		}
		f.queues[peer] = count
		f.queued[hash] = op
		f.queue.Push(op, -int64(block.NumberU64()))
		if f.queueChangeHook != nil {
			f.queueChangeHook(op.block.Hash(), true)
		}
		log.Debug("Queued propagated block", "peer", peer, "number", block.Number(), "hash", hash, "queued", f.queue.Size())
	}
}

//insert生成新的goroutine以在链中执行块插入。如果
//块的编号与当前导入阶段的高度相同，它将更新
//相应地，相位状态。
func (f *Fetcher) insert(peer string, block *types.Block) {
	hash := block.Hash()

//在新线程上运行导入
	log.Debug("Importing propagated block", "peer", peer, "number", block.Number(), "hash", hash)
	go func() {
		defer func() { f.done <- hash }()

//如果父级未知，则中止插入
		parent := f.getBlock(block.ParentHash())
		if parent == nil {
			log.Debug("Unknown parent of propagated block", "peer", peer, "number", block.Number(), "hash", hash, "parent", block.ParentHash())
			return
		}
//快速验证头并在块通过时传播该块
		switch err := f.verifyHeader(block.Header()); err {
		case nil:
//一切正常，迅速传播给我们的同行
			propBroadcastOutTimer.UpdateSince(block.ReceivedAt)
			go f.broadcastBlock(block, true)

		case consensus.ErrFutureBlock:
//奇怪的未来块，不要失败，但都不会传播

		default:
//Something went very wrong, drop the peer
			log.Debug("Propagated block verification failed", "peer", peer, "number", block.Number(), "hash", hash, "err", err)
			f.dropPeer(peer)
			return
		}
//运行实际导入并记录所有问题
		if _, err := f.insertChain(types.Blocks{block}); err != nil {
			log.Debug("Propagated block import failed", "peer", peer, "number", block.Number(), "hash", hash, "err", err)
			return
		}
//如果导入成功，则广播块
		propAnnounceOutTimer.UpdateSince(block.ReceivedAt)
		go f.broadcastBlock(block, false)

//如果需要，调用测试挂钩
		if f.importedHook != nil {
			f.importedHook(block)
		}
	}()
}

//遗忘哈希从提取程序中删除块通知的所有跟踪
//内部状态。
func (f *Fetcher) forgetHash(hash common.Hash) {
//删除所有挂起的公告和递减DOS计数器
	for _, announce := range f.announced[hash] {
		f.announces[announce.origin]--
		if f.announces[announce.origin] == 0 {
			delete(f.announces, announce.origin)
		}
	}
	delete(f.announced, hash)
	if f.announceChangeHook != nil {
		f.announceChangeHook(hash, false)
	}
//删除所有挂起的提取并减少DOS计数器
	if announce := f.fetching[hash]; announce != nil {
		f.announces[announce.origin]--
		if f.announces[announce.origin] == 0 {
			delete(f.announces, announce.origin)
		}
		delete(f.fetching, hash)
	}

//删除所有挂起的完成请求并减少DOS计数器
	for _, announce := range f.fetched[hash] {
		f.announces[announce.origin]--
		if f.announces[announce.origin] == 0 {
			delete(f.announces, announce.origin)
		}
	}
	delete(f.fetched, hash)

//删除所有挂起的完成并减少DOS计数器
	if announce := f.completing[hash]; announce != nil {
		f.announces[announce.origin]--
		if f.announces[announce.origin] == 0 {
			delete(f.announces, announce.origin)
		}
		delete(f.completing, hash)
	}
}

//CISTION块从取回器的内部移除队列块的所有踪迹。
//状态。
func (f *Fetcher) forgetBlock(hash common.Hash) {
	if insert := f.queued[hash]; insert != nil {
		f.queues[insert.origin]--
		if f.queues[insert.origin] == 0 {
			delete(f.queues, insert.origin)
		}
		delete(f.queued, hash)
	}
}
