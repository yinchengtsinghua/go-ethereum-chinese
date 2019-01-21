
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

//包les实现轻以太坊子协议。
package les

import (
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/light"
	"github.com/ethereum/go-ethereum/log"
)

const (
blockDelayTimeout    = time.Second * 10 //对等机宣布已被其他人确认的头的超时
maxNodeCount         = 20               //为每个对等机记住的最大fetchertreenode条目数
serverStateAvailable = 100              //假定状态可用性的最近块数
)

//LightFetcher实现对新公布的头的检索。它还为
//ODR系统确保我们只向已经处理过的对等方请求与某个块相关的数据
//并宣布封锁。
type lightFetcher struct {
	pm    *ProtocolManager
	odr   *LesOdr
	chain *light.LightChain

lock            sync.Mutex //锁保护对提取程序内部状态变量（发送的请求除外）的访问
	maxConfirmedTd  *big.Int
	peers           map[*peer]*fetcherPeerInfo
	lastUpdateStats *updateStatsEntry
	syncing         bool
	syncDone        chan *peer

reqMu      sync.RWMutex //reqmu保护对发送的头提取请求的访问
	requested  map[uint64]fetchRequest
	deliverChn chan fetchResponse
	timeoutChn chan uint64
requestChn chan bool //如果从外部启动，则为真
}

//FETCHEPERIVENT保存关于每个活动对等点的特定的信息
type fetcherPeerInfo struct {
	root, lastAnnounced *fetcherTreeNode
	nodeCnt             int
	confirmedTd         *big.Int
	bestConfirmed       *fetcherTreeNode
	nodeByHash          map[common.Hash]*fetcherTreeNode
	firstUpdateStats    *updateStatsEntry
}

//fetchergreenode是树的一个节点，最近保存了关于块的信息。
//announced and confirmed by a certain peer. Each new announce message from a peer
//将节点添加到树中，基于先前公布的头部和重新排列深度。
//树节点有三种可能的状态：
//-已宣布：尚未下载（已知），但我们知道它的头、编号和td
//-中间：不知道，散列和td为空，当已知时填写。
//-已知：由该对等方宣布并下载（从任何对等方）。
//这种结构可以始终知道哪个对等机具有某个块，
//这对于为ODR请求以及
//将新的头封为圣徒。它也有助于始终下载所需的最少数量
//具有单个请求的头的数量。
type fetcherTreeNode struct {
	hash             common.Hash
	number           uint64
	td               *big.Int
	known, requested bool
	parent           *fetcherTreeNode
	children         []*fetcherTreeNode
}

//fetchRequest表示头下载请求
type fetchRequest struct {
	hash    common.Hash
	amount  uint64
	peer    *peer
	sent    mclock.AbsTime
	timeout bool
}

//fetchResponse表示头下载响应
type fetchResponse struct {
	reqID   uint64
	headers []*types.Header
	peer    *peer
}

//new light fetcher创建新的light fetcher
func newLightFetcher(pm *ProtocolManager) *lightFetcher {
	f := &lightFetcher{
		pm:             pm,
		chain:          pm.blockchain.(*light.LightChain),
		odr:            pm.odr,
		peers:          make(map[*peer]*fetcherPeerInfo),
		deliverChn:     make(chan fetchResponse, 100),
		requested:      make(map[uint64]fetchRequest),
		timeoutChn:     make(chan uint64),
		requestChn:     make(chan bool, 100),
		syncDone:       make(chan *peer),
		maxConfirmedTd: big.NewInt(0),
	}
	pm.peers.notify(f)

	f.pm.wg.Add(1)
	go f.syncLoop()
	return f
}

//同步循环是取光器的主事件循环
func (f *lightFetcher) syncLoop() {
	requesting := false
	defer f.pm.wg.Done()
	for {
		select {
		case <-f.pm.quitSync:
			return
//当接收到新的公告时，请求循环将一直运行，直到
//无需或可能进一步请求
		case newAnnounce := <-f.requestChn:
			f.lock.Lock()
			s := requesting
			requesting = false
			var (
				rq      *distReq
				reqID   uint64
				syncing bool
			)
			if !f.syncing && !(newAnnounce && s) {
				rq, reqID, syncing = f.nextRequest()
			}
			f.lock.Unlock()

			if rq != nil {
				requesting = true
				if _, ok := <-f.pm.reqDist.queue(rq); ok {
					if syncing {
						f.lock.Lock()
						f.syncing = true
						f.lock.Unlock()
					} else {
						go func() {
							time.Sleep(softRequestTimeout)
							f.reqMu.Lock()
							req, ok := f.requested[reqID]
							if ok {
								req.timeout = true
								f.requested[reqID] = req
							}
							f.reqMu.Unlock()
//尽可能继续启动新请求
							f.requestChn <- false
						}()
					}
				} else {
					f.requestChn <- false
				}
			}
		case reqID := <-f.timeoutChn:
			f.reqMu.Lock()
			req, ok := f.requested[reqID]
			if ok {
				delete(f.requested, reqID)
			}
			f.reqMu.Unlock()
			if ok {
				f.pm.serverPool.adjustResponseTime(req.peer.poolEntry, time.Duration(mclock.Now()-req.sent), true)
				req.peer.Log().Debug("Fetching data timed out hard")
				go f.pm.removePeer(req.peer.id)
			}
		case resp := <-f.deliverChn:
			f.reqMu.Lock()
			req, ok := f.requested[resp.reqID]
			if ok && req.peer != resp.peer {
				ok = false
			}
			if ok {
				delete(f.requested, resp.reqID)
			}
			f.reqMu.Unlock()
			if ok {
				f.pm.serverPool.adjustResponseTime(req.peer.poolEntry, time.Duration(mclock.Now()-req.sent), req.timeout)
			}
			f.lock.Lock()
			if !ok || !(f.syncing || f.processResponse(req, resp)) {
				resp.peer.Log().Debug("Failed processing response")
				go f.pm.removePeer(resp.peer.id)
			}
			f.lock.Unlock()
		case p := <-f.syncDone:
			f.lock.Lock()
			p.Log().Debug("Done synchronising with peer")
			f.checkSyncedHeaders(p)
			f.syncing = false
			f.lock.Unlock()
			f.requestChn <- false
		}
	}
}

//registerpeer将新的对等添加到提取程序的对等集
func (f *lightFetcher) registerPeer(p *peer) {
	p.lock.Lock()
	p.hasBlock = func(hash common.Hash, number uint64, hasState bool) bool {
		return f.peerHasBlock(p, hash, number, hasState)
	}
	p.lock.Unlock()

	f.lock.Lock()
	defer f.lock.Unlock()

	f.peers[p] = &fetcherPeerInfo{nodeByHash: make(map[common.Hash]*fetcherTreeNode)}
}

//UnregisterPeer从提取程序的对等集中删除一个新对等。
func (f *lightFetcher) unregisterPeer(p *peer) {
	p.lock.Lock()
	p.hasBlock = nil
	p.lock.Unlock()

	f.lock.Lock()
	defer f.lock.Unlock()

//检查潜在的超时块延迟统计信息
	f.checkUpdateStats(p, nil)
	delete(f.peers, p)
}

//公告处理从对等端接收的新公告消息，添加新的
//节点到对等机的块树，并在必要时删除旧节点
func (f *lightFetcher) announce(p *peer, head *announceData) {
	f.lock.Lock()
	defer f.lock.Unlock()
	p.Log().Debug("Received new announcement", "number", head.Number, "hash", head.Hash, "reorg", head.ReorgDepth)

	fp := f.peers[p]
	if fp == nil {
		p.Log().Debug("Announcement from unknown peer")
		return
	}

	if fp.lastAnnounced != nil && head.Td.Cmp(fp.lastAnnounced.td) <= 0 {
//公布的技术数据应该严格单调
		p.Log().Debug("Received non-monotonic td", "current", head.Td, "previous", fp.lastAnnounced.td)
		go f.pm.removePeer(p.id)
		return
	}

	n := fp.lastAnnounced
	for i := uint64(0); i < head.ReorgDepth; i++ {
		if n == nil {
			break
		}
		n = n.parent
	}
//n现在是reorg的共同祖先，添加一个新的节点分支
	if n != nil && (head.Number >= n.number+maxNodeCount || head.Number <= n.number) {
//如果公布的头块高度低于或等于N或太远无法添加
//中间节点然后放弃先前的通知信息并触发重新同步
		n = nil
		fp.nodeCnt = 0
		fp.nodeByHash = make(map[common.Hash]*fetcherTreeNode)
	}
	if n != nil {
//检查节点计数是否太高，无法添加新节点，必要时丢弃最旧的节点
		locked := false
		for uint64(fp.nodeCnt)+head.Number-n.number > maxNodeCount && fp.root != nil {
			if !locked {
				f.chain.LockChain()
				defer f.chain.UnlockChain()
				locked = true
			}
//如果根的一个子级是规范的，请保留它，删除其他分支和根本身。
			var newRoot *fetcherTreeNode
			for i, nn := range fp.root.children {
				if rawdb.ReadCanonicalHash(f.pm.chainDb, nn.number) == nn.hash {
					fp.root.children = append(fp.root.children[:i], fp.root.children[i+1:]...)
					nn.parent = nil
					newRoot = nn
					break
				}
			}
			fp.deleteNode(fp.root)
			if n == fp.root {
				n = newRoot
			}
			fp.root = newRoot
			if newRoot == nil || !f.checkKnownNode(p, newRoot) {
				fp.bestConfirmed = nil
				fp.confirmedTd = nil
			}

			if n == nil {
				break
			}
		}
		if n != nil {
			for n.number < head.Number {
				nn := &fetcherTreeNode{number: n.number + 1, parent: n}
				n.children = append(n.children, nn)
				n = nn
				fp.nodeCnt++
			}
			n.hash = head.Hash
			n.td = head.Td
			fp.nodeByHash[n.hash] = n
		}
	}
	if n == nil {
//找不到REORG公共祖先或必须删除整个树，需要新的根目录和重新同步
		if fp.root != nil {
			fp.deleteNode(fp.root)
		}
		n = &fetcherTreeNode{hash: head.Hash, number: head.Number, td: head.Td}
		fp.root = n
		fp.nodeCnt++
		fp.nodeByHash[n.hash] = n
		fp.bestConfirmed = nil
		fp.confirmedTd = nil
	}

	f.checkKnownNode(p, n)
	p.lock.Lock()
	p.headInfo = head
	fp.lastAnnounced = n
	p.lock.Unlock()
	f.checkUpdateStats(p, nil)
	f.requestChn <- true
}

//如果我们可以假设对等方知道给定的块，那么peerhassblock返回true。
//根据它的公告
func (f *lightFetcher) peerHasBlock(p *peer, hash common.Hash, number uint64, hasState bool) bool {
	f.lock.Lock()
	defer f.lock.Unlock()

	fp := f.peers[p]
	if fp == nil || fp.root == nil {
		return false
	}

	if hasState {
		if fp.lastAnnounced == nil || fp.lastAnnounced.number > number+serverStateAvailable {
			return false
		}
	}

	if f.syncing {
//同步时始终返回true
//误报是可以接受的，一个更复杂的条件可以稍后实现。
		return true
	}

	if number >= fp.root.number {
//如果知道它，它应该在对等机的块树中，这已经足够新了。
		return fp.nodeByHash[hash] != nil
	}
	f.chain.LockChain()
	defer f.chain.UnlockChain()
//如果它比对等的块树根还老，但它在同一个规范链中
//作为根，我们仍然可以确定同伴知道它。
//
//同步时，只要检查它是否是已知链的一部分，就没有比我们更好的了
//可以，因为我们还不知道最新的块哈希
	return rawdb.ReadCanonicalHash(f.pm.chainDb, fp.root.number) == fp.root.hash && rawdb.ReadCanonicalHash(f.pm.chainDb, number) == hash
}

//RequestAmount从开始计算要下载的头的数量
//从某个头向后
func (f *lightFetcher) requestAmount(p *peer, n *fetcherTreeNode) uint64 {
	amount := uint64(0)
	nn := n
	for nn != nil && !f.checkKnownNode(p, nn) {
		nn = nn.parent
		amount++
	}
	if nn == nil {
		amount = n.number
	}
	return amount
}

//REQUESTEDID指示获取程序是否已请求某个REQID
func (f *lightFetcher) requestedID(reqID uint64) bool {
	f.reqMu.RLock()
	_, ok := f.requested[reqID]
	f.reqMu.RUnlock()
	return ok
}

//nextrequest选择要请求的对等端和公告头，下一个，amount
//从头部开始向后下载也会返回
func (f *lightFetcher) nextRequest() (*distReq, uint64, bool) {
	var (
		bestHash   common.Hash
		bestAmount uint64
	)
	bestTd := f.maxConfirmedTd
	bestSyncing := false

	for p, fp := range f.peers {
		for hash, n := range fp.nodeByHash {
			if !f.checkKnownNode(p, n) && !n.requested && (bestTd == nil || n.td.Cmp(bestTd) >= 0) {
				amount := f.requestAmount(p, n)
				if bestTd == nil || n.td.Cmp(bestTd) > 0 || amount < bestAmount {
					bestHash = hash
					bestAmount = amount
					bestTd = n.td
					bestSyncing = fp.bestConfirmed == nil || fp.root == nil || !f.checkKnownNode(p, fp.root)
				}
			}
		}
	}
	if bestTd == f.maxConfirmedTd {
		return nil, 0, false
	}

	var rq *distReq
	reqID := genReqID()
	if bestSyncing {
		rq = &distReq{
			getCost: func(dp distPeer) uint64 {
				return 0
			},
			canSend: func(dp distPeer) bool {
				p := dp.(*peer)
				f.lock.Lock()
				defer f.lock.Unlock()

				fp := f.peers[p]
				return fp != nil && fp.nodeByHash[bestHash] != nil
			},
			request: func(dp distPeer) func() {
				go func() {
					p := dp.(*peer)
					p.Log().Debug("Synchronisation started")
					f.pm.synchronise(p)
					f.syncDone <- p
				}()
				return nil
			},
		}
	} else {
		rq = &distReq{
			getCost: func(dp distPeer) uint64 {
				p := dp.(*peer)
				return p.GetRequestCost(GetBlockHeadersMsg, int(bestAmount))
			},
			canSend: func(dp distPeer) bool {
				p := dp.(*peer)
				f.lock.Lock()
				defer f.lock.Unlock()

				fp := f.peers[p]
				if fp == nil {
					return false
				}
				n := fp.nodeByHash[bestHash]
				return n != nil && !n.requested
			},
			request: func(dp distPeer) func() {
				p := dp.(*peer)
				f.lock.Lock()
				fp := f.peers[p]
				if fp != nil {
					n := fp.nodeByHash[bestHash]
					if n != nil {
						n.requested = true
					}
				}
				f.lock.Unlock()

				cost := p.GetRequestCost(GetBlockHeadersMsg, int(bestAmount))
				p.fcServer.QueueRequest(reqID, cost)
				f.reqMu.Lock()
				f.requested[reqID] = fetchRequest{hash: bestHash, amount: bestAmount, peer: p, sent: mclock.Now()}
				f.reqMu.Unlock()
				go func() {
					time.Sleep(hardRequestTimeout)
					f.timeoutChn <- reqID
				}()
				return func() { p.RequestHeadersByHash(reqID, cost, bestHash, int(bestAmount), 0, true) }
			},
		}
	}
	return rq, reqID, bestSyncing
}

//DeliverHeaders传递要处理的头下载请求响应
func (f *lightFetcher) deliverHeaders(peer *peer, reqID uint64, headers []*types.Header) {
	f.deliverChn <- fetchResponse{reqID: reqID, headers: headers, peer: peer}
}

//processResponse处理头下载请求响应，如果成功，则返回true
func (f *lightFetcher) processResponse(req fetchRequest, resp fetchResponse) bool {
	if uint64(len(resp.headers)) != req.amount || resp.headers[0].Hash() != req.hash {
		req.peer.Log().Debug("Response content mismatch", "requested", len(resp.headers), "reqfrom", resp.headers[0], "delivered", req.amount, "delfrom", req.hash)
		return false
	}
	headers := make([]*types.Header, req.amount)
	for i, header := range resp.headers {
		headers[int(req.amount)-1-i] = header
	}
	if _, err := f.chain.InsertHeaderChain(headers, 1); err != nil {
		if err == consensus.ErrFutureBlock {
			return true
		}
		log.Debug("Failed to insert header chain", "err", err)
		return false
	}
	tds := make([]*big.Int, len(headers))
	for i, header := range headers {
		td := f.chain.GetTd(header.Hash(), header.Number.Uint64())
		if td == nil {
			log.Debug("Total difficulty not found for header", "index", i+1, "number", header.Number, "hash", header.Hash())
			return false
		}
		tds[i] = td
	}
	f.newHeaders(headers, tds)
	return true
}

//newheaders根据
//下载并验证批或头
func (f *lightFetcher) newHeaders(headers []*types.Header, tds []*big.Int) {
	var maxTd *big.Int
	for p, fp := range f.peers {
		if !f.checkAnnouncedHeaders(fp, headers, tds) {
			p.Log().Debug("Inconsistent announcement")
			go f.pm.removePeer(p.id)
		}
		if fp.confirmedTd != nil && (maxTd == nil || maxTd.Cmp(fp.confirmedTd) > 0) {
			maxTd = fp.confirmedTd
		}
	}
	if maxTd != nil {
		f.updateMaxConfirmedTd(maxTd)
	}
}

//如果验证后需要，checkAnnouncedHeaders会更新对等方的块树
//一批头文件。它搜索具有
//matching tree node (if any), and if it has not been marked as known already,
//将其及其父级设置为已知（甚至那些比当前
//已验证）。返回值显示所有哈希、数字和TDS是否匹配
//正确到公布的值（否则应删除对等机）。
func (f *lightFetcher) checkAnnouncedHeaders(fp *fetcherPeerInfo, headers []*types.Header, tds []*big.Int) bool {
	var (
		n      *fetcherTreeNode
		header *types.Header
		td     *big.Int
	)

	for i := len(headers) - 1; ; i-- {
		if i < 0 {
			if n == nil {
//没有更多的标题，也没有要匹配的内容
				return true
			}
//最近传递的头已用完，但尚未到达此对等方已知的节点，请继续匹配
			hash, number := header.ParentHash, header.Number.Uint64()-1
			td = f.chain.GetTd(hash, number)
			header = f.chain.GetHeader(hash, number)
			if header == nil || td == nil {
				log.Error("Missing parent of validated header", "hash", hash, "number", number)
				return false
			}
		} else {
			header = headers[i]
			td = tds[i]
		}
		hash := header.Hash()
		number := header.Number.Uint64()
		if n == nil {
			n = fp.nodeByHash[hash]
		}
		if n != nil {
			if n.td == nil {
//节点未通知
				if nn := fp.nodeByHash[hash]; nn != nil {
//如果已经有一个具有相同哈希的节点，请继续执行该操作并删除该节点。
					nn.children = append(nn.children, n.children...)
					n.children = nil
					fp.deleteNode(n)
					n = nn
				} else {
					n.hash = hash
					n.td = td
					fp.nodeByHash[hash] = n
				}
			}
//检查它是否与标题匹配
			if n.hash != hash || n.number != number || n.td.Cmp(td) != 0 {
//对等机以前发出了无效的通知
				return false
			}
			if n.known {
//我们到达了一个与我们的期望相符的已知节点，成功地返回
				return true
			}
			n.known = true
			if fp.confirmedTd == nil || td.Cmp(fp.confirmedTd) > 0 {
				fp.confirmedTd = td
				fp.bestConfirmed = n
			}
			n = n.parent
			if n == nil {
				return true
			}
		}
	}
}

//checkSyncedHeaders通过标记在同步后更新对等方的块树
//下载了已知的邮件头。如果在
//syncing, the peer is dropped.
func (f *lightFetcher) checkSyncedHeaders(p *peer) {
	fp := f.peers[p]
	if fp == nil {
		p.Log().Debug("Unknown peer to check sync headers")
		return
	}
	n := fp.lastAnnounced
	var td *big.Int
	for n != nil {
		if td = f.chain.GetTd(n.hash, n.number); td != nil {
			break
		}
		n = n.parent
	}
//现在n是同步后最新下载的头文件
	if n == nil {
		p.Log().Debug("Synchronisation failed")
		go f.pm.removePeer(p.id)
	} else {
		header := f.chain.GetHeader(n.hash, n.number)
		f.newHeaders([]*types.Header{header}, []*big.Int{td})
	}
}

//checkknownnode检查是否已知块树节点（已下载并验证）
//如果以前不知道，但在数据库中找到，则设置其已知标志
func (f *lightFetcher) checkKnownNode(p *peer, n *fetcherTreeNode) bool {
	if n.known {
		return true
	}
	td := f.chain.GetTd(n.hash, n.number)
	if td == nil {
		return false
	}
	header := f.chain.GetHeader(n.hash, n.number)
//检查header和td的可用性，因为chain db mutex不保护读操作
//注意：返回false在这里总是安全的
	if header == nil {
		return false
	}

	fp := f.peers[p]
	if fp == nil {
		p.Log().Debug("Unknown peer to check known nodes")
		return false
	}
	if !f.checkAnnouncedHeaders(fp, []*types.Header{header}, []*big.Int{td}) {
		p.Log().Debug("Inconsistent announcement")
		go f.pm.removePeer(p.id)
	}
	if fp.confirmedTd != nil {
		f.updateMaxConfirmedTd(fp.confirmedTd)
	}
	return n.known
}

//删除节点从对等块树中删除节点及其子树
func (fp *fetcherPeerInfo) deleteNode(n *fetcherTreeNode) {
	if n.parent != nil {
		for i, nn := range n.parent.children {
			if nn == n {
				n.parent.children = append(n.parent.children[:i], n.parent.children[i+1:]...)
				break
			}
		}
	}
	for {
		if n.td != nil {
			delete(fp.nodeByHash, n.hash)
		}
		fp.nodeCnt--
		if len(n.children) == 0 {
			return
		}
		for i, nn := range n.children {
			if i == 0 {
				n = nn
			} else {
				fp.deleteNode(nn)
			}
		}
	}
}

//updateStatentEntry项形成一个链接列表，每次具有更高TD的新头时，都会使用新项展开该列表。
//已下载并验证。该列表包含一系列已确认的最大td值
//这些值被确认的时间，都是单调增加的。计算最大确认td
//无论是全球范围内的所有同行，还是每个单独的同行（也就是说，给定的同行已经宣布了领导
//它也已经从任何一个对等机上下载，无论是在发布之前还是之后）。
//链接列表有一个全局尾部，其中添加了新的已确认TD条目，并为每个对等端分别添加了一个头部，
//pointing to the next Td entry that is higher than the peer's max confirmed Td (nil if it has already confirmed
//目前的全球负责人）。
type updateStatsEntry struct {
	time mclock.AbsTime
	td   *big.Int
	next *updateStatsEntry
}

//updateMaxConfirmedtd更新活动对等机的块延迟统计信息。一旦确定了新的最高TD，
//将其与确认时间一起添加到链接列表的末尾。然后检查哪些同行
//已经确认了一个具有相同或更高TD（计算为零块延迟）的头，并更新了他们的统计数据。
//现在还没有确认该头的人将通过随后的checkupdatestats调用更新
//positive block delay value.
func (f *lightFetcher) updateMaxConfirmedTd(td *big.Int) {
	if f.maxConfirmedTd == nil || td.Cmp(f.maxConfirmedTd) > 0 {
		f.maxConfirmedTd = td
		newEntry := &updateStatsEntry{
			time: mclock.Now(),
			td:   td,
		}
		if f.lastUpdateStats != nil {
			f.lastUpdateStats.next = newEntry
		}
		f.lastUpdateStats = newEntry
		for p := range f.peers {
			f.checkUpdateStats(p, newEntry)
		}
	}
}

//CheckUpdateStats检查那些在确认某个最高TD（或更大TD）时还没有确认的同行。
//已被另一个对等方确认。如果他们现在已经确认了这样一个头部，他们的统计数据将更新为
//阻塞延迟，即（此对等方的确认时间）-（第一次确认时间）。BlockDelayTimeout通过后，
//统计信息将用BlockDelayTimeout值更新。在这两种情况下，已确认或超时的更新StatsEntry
//项目将从链接列表的标题中删除。
//如果新条目已添加到全局尾部，则在此处作为参数传递，即使此函数
//假设它已经被添加，这样如果对等方的列表是空的（所有头都已确认，头为零）。
//它可以将新头设置为newentry。
func (f *lightFetcher) checkUpdateStats(p *peer, newEntry *updateStatsEntry) {
	now := mclock.Now()
	fp := f.peers[p]
	if fp == nil {
		p.Log().Debug("Unknown peer to check update stats")
		return
	}
	if newEntry != nil && fp.firstUpdateStats == nil {
		fp.firstUpdateStats = newEntry
	}
	for fp.firstUpdateStats != nil && fp.firstUpdateStats.time <= now-mclock.AbsTime(blockDelayTimeout) {
		f.pm.serverPool.adjustBlockDelay(p.poolEntry, blockDelayTimeout)
		fp.firstUpdateStats = fp.firstUpdateStats.next
	}
	if fp.confirmedTd != nil {
		for fp.firstUpdateStats != nil && fp.firstUpdateStats.td.Cmp(fp.confirmedTd) <= 0 {
			f.pm.serverPool.adjustBlockDelay(p.poolEntry, time.Duration(now-fp.firstUpdateStats.time))
			fp.firstUpdateStats = fp.firstUpdateStats.next
		}
	}
}
