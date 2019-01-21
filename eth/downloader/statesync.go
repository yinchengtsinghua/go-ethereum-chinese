
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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

package downloader

import (
	"fmt"
	"hash"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/trie"
	"golang.org/x/crypto/sha3"
)

//statereq表示一批状态获取请求，分组到
//单个数据检索网络包。
type stateReq struct {
items    []common.Hash              //要下载的状态项的哈希
tasks    map[common.Hash]*stateTask //下载任务以跟踪以前的尝试
timeout  time.Duration              //Maximum round trip time for this to complete
timer    *time.Timer                //RTT超时过期时要触发的计时器
peer     *peerConnection            //我们请求的同伴
response [][]byte                   //对等机的响应数据（超时为零）
dropped  bool                       //标记对等机是否提前退出
}

//如果此请求超时，则返回timed out。
func (req *stateReq) timedOut() bool {
	return req.response == nil
}

//StateSyncStats是状态检索期间要报告的进度统计信息的集合。
//同步到RPC请求并显示在用户日志中。
type stateSyncStats struct {
processed  uint64 //处理的状态条目数
duplicate  uint64 //两次下载的状态条目数
unexpected uint64 //接收到的非请求状态条目数
pending    uint64 //仍挂起状态条目数
}

//SyncState开始使用给定的根哈希下载状态。
func (d *Downloader) syncState(root common.Hash) *stateSync {
	s := newStateSync(d, root)
	select {
	case d.stateSyncStart <- s:
	case <-d.quitCh:
		s.err = errCancelStateFetch
		close(s.done)
	}
	return s
}

//stateFetcher manages the active state sync and accepts requests
//代表它。
func (d *Downloader) stateFetcher() {
	for {
		select {
		case s := <-d.stateSyncStart:
			for next := s; next != nil; {
				next = d.runStateSync(next)
			}
		case <-d.stateCh:
//不运行同步时忽略状态响应。
		case <-d.quitCh:
			return
		}
	}
}

//runStateSync runs a state synchronisation until it completes or another root
//请求将哈希切换到。
func (d *Downloader) runStateSync(s *stateSync) *stateSync {
	var (
active   = make(map[string]*stateReq) //当前飞行请求
finished []*stateReq                  //已完成或失败的请求
timeout  = make(chan *stateReq)       //活动请求超时
	)
	defer func() {
//退出时取消活动请求计时器。还可以将对等机设置为空闲，以便
//可用于下一次同步。
		for _, req := range active {
			req.timer.Stop()
			req.peer.SetNodeDataIdle(len(req.items))
		}
	}()
//运行状态同步。
	go s.run()
	defer s.Cancel()

//倾听同伴离开事件以取消分配的任务
	peerDrop := make(chan *peerConnection, 1024)
	peerSub := s.d.peers.SubscribePeerDrops(peerDrop)
	defer peerSub.Unsubscribe()

	for {
//如果有第一个缓冲元素，则启用发送。
		var (
			deliverReq   *stateReq
			deliverReqCh chan *stateReq
		)
		if len(finished) > 0 {
			deliverReq = finished[0]
			deliverReqCh = s.deliver
		}

		select {
//The stateSync lifecycle:
		case next := <-d.stateSyncStart:
			return next

		case <-s.done:
			return nil

//将下一个完成的请求发送到当前同步：
		case deliverReqCh <- deliverReq:
//移出第一个请求，但也为GC将空槽设置为零
			copy(finished, finished[1:])
			finished[len(finished)-1] = nil
			finished = finished[:len(finished)-1]

//处理传入状态包：
		case pack := <-d.stateCh:
//放弃任何未请求的数据（或以前超时的数据）
			req := active[pack.PeerId()]
			if req == nil {
				log.Debug("Unrequested node data", "peer", pack.PeerId(), "len", pack.Items())
				continue
			}
//完成请求并排队等待处理
			req.timer.Stop()
			req.response = pack.(*statePack).states

			finished = append(finished, req)
			delete(active, pack.PeerId())

//处理掉的对等连接：
		case p := <-peerDrop:
//Skip if no request is currently pending
			req := active[p.id]
			if req == nil {
				continue
			}
//完成请求并排队等待处理
			req.timer.Stop()
			req.dropped = true

			finished = append(finished, req)
			delete(active, p.id)

//处理超时请求：
		case req := <-timeout:
//如果对等机已经在请求其他东西，请忽略过时的超时。
//当超时和传递同时发生时，就会发生这种情况，
//导致两种途径触发。
			if active[req.peer.id] != req {
				continue
			}
//将超时数据移回下载队列
			finished = append(finished, req)
			delete(active, req.peer.id)

//Track outgoing state requests:
		case req := <-d.trackStateReq:
//如果此对等机已经存在活动请求，则说明存在问题。在
//理论上，trie节点调度决不能将两个请求分配给同一个
//同龄人。然而，在实践中，对等端可能会收到一个请求，断开连接并
//在前一次超时前立即重新连接。在这种情况下，第一个
//请求永远不会得到满足，唉，我们不能悄悄地改写它，就像这样。
//导致有效的请求丢失，并同步卡住。
			if old := active[req.peer.id]; old != nil {
				log.Warn("Busy peer assigned new state fetch", "peer", old.peer.id)

//确保前一个不会被误丢
				old.timer.Stop()
				old.dropped = true

				finished = append(finished, old)
			}
//Start a timer to notify the sync loop if the peer stalled.
			req.timer = time.AfterFunc(req.timeout, func() {
				select {
				case timeout <- req:
				case <-s.done:
//Prevent leaking of timer goroutines in the unlikely case where a
//在退出runstatesync之前，计时器将被激发。
				}
			})
			active[req.peer.id] = req
		}
	}
}

//stateSync schedules requests for downloading a particular state trie defined
//通过给定的状态根。
type stateSync struct {
d *Downloader //用于访问和管理当前对等集的下载程序实例

sched  *trie.Sync                 //State trie sync scheduler defining the tasks
keccak hash.Hash                  //KECCAK256哈希验证交付
tasks  map[common.Hash]*stateTask //当前排队等待检索的任务集

	numUncommitted   int
	bytesUncommitted int

deliver    chan *stateReq //传递通道多路复用对等响应
cancel     chan struct{}  //发送终止请求信号的通道
cancelOnce sync.Once      //确保Cancel只被调用一次
done       chan struct{}  //通道到信号终止完成
err        error          //同步期间发生的任何错误（在完成前设置）
}

//statetask表示单个trie节点下载任务，包含一组
//peers already attempted retrieval from to detect stalled syncs and abort.
type stateTask struct {
	attempts map[string]struct{}
}

//newstatesync创建新的状态trie下载计划程序。此方法不
//开始同步。用户需要调用run来启动。
func newStateSync(d *Downloader, root common.Hash) *stateSync {
	return &stateSync{
		d:       d,
		sched:   state.NewStateSync(root, d.stateDB),
		keccak:  sha3.NewLegacyKeccak256(),
		tasks:   make(map[common.Hash]*stateTask),
		deliver: make(chan *stateReq),
		cancel:  make(chan struct{}),
		done:    make(chan struct{}),
	}
}

//run starts the task assignment and response processing loop, blocking until
//它结束，并最终通知等待循环的任何Goroutines
//完成。
func (s *stateSync) run() {
	s.err = s.loop()
	close(s.done)
}

//Wait blocks until the sync is done or canceled.
func (s *stateSync) Wait() error {
	<-s.done
	return s.err
}

//取消取消同步并等待其关闭。
func (s *stateSync) Cancel() error {
	s.cancelOnce.Do(func() { close(s.cancel) })
	return s.Wait()
}

//循环是状态trie-sync的主事件循环。它负责
//assignment of new tasks to peers (including sending it to them) as well as
//用于处理入站数据。注意，循环不直接
//从对等端接收数据，而不是在下载程序中缓冲这些数据，
//按这里异步。原因是将处理与数据接收分离
//超时。
func (s *stateSync) loop() (err error) {
//侦听新的对等事件以将任务分配给它们
	newPeer := make(chan *peerConnection, 1024)
	peerSub := s.d.peers.SubscribeNewPeers(newPeer)
	defer peerSub.Unsubscribe()
	defer func() {
		cerr := s.commit(true)
		if err == nil {
			err = cerr
		}
	}()

//继续分配新任务，直到同步完成或中止
	for s.sched.Pending() > 0 {
		if err = s.commit(false); err != nil {
			return err
		}
		s.assignTasks()
//分配的任务，等待发生什么
		select {
		case <-newPeer:
//新对等机已到达，请尝试分配它的下载任务

		case <-s.cancel:
			return errCancelStateFetch

		case <-s.d.cancelCh:
			return errCancelStateFetch

		case req := <-s.deliver:
//响应、断开连接或超时触发，如果停止，则丢弃对等机
			log.Trace("Received node data response", "peer", req.peer.id, "count", len(req.response), "dropped", req.dropped, "timeout", !req.dropped && req.timedOut())
			if len(req.items) <= 2 && !req.dropped && req.timedOut() {
//2项是最低要求，即使超时，我们也没有用
//现在这个人。
				log.Warn("Stalling state sync, dropping peer", "peer", req.peer.id)
				s.d.dropPeer(req.peer.id)
			}
//处理所有接收到的Blob并检查是否存在过时的传递
			delivered, err := s.process(req)
			if err != nil {
				log.Warn("Node data write error", "err", err)
				return err
			}
			req.peer.SetNodeDataIdle(delivered)
		}
	}
	return nil
}

func (s *stateSync) commit(force bool) error {
	if !force && s.bytesUncommitted < ethdb.IdealBatchSize {
		return nil
	}
	start := time.Now()
	b := s.d.stateDB.NewBatch()
	if written, err := s.sched.Commit(b); written == 0 || err != nil {
		return err
	}
	if err := b.Write(); err != nil {
		return fmt.Errorf("DB write error: %v", err)
	}
	s.updateStats(s.numUncommitted, 0, 0, time.Since(start))
	s.numUncommitted = 0
	s.bytesUncommitted = 0
	return nil
}

//assign tasks尝试将新任务分配给所有空闲对等端，或者从
//当前正在重试批处理，或者从TIE同步本身获取新数据。
func (s *stateSync) assignTasks() {
//遍历所有空闲对等点，并尝试为其分配状态获取
	peers, _ := s.d.peers.NodeDataIdlePeers()
	for _, p := range peers {
//分配一批与估计的延迟/带宽成比例的获取
		cap := p.NodeDataCapacity(s.d.requestRTT())
		req := &stateReq{peer: p, timeout: s.d.requestTTL()}
		s.fillTasks(cap, req)

//如果为对等机分配了要获取的任务，则发送网络请求
		if len(req.items) > 0 {
			req.peer.log.Trace("Requesting new batch of data", "type", "state", "count", len(req.items))
			select {
			case s.d.trackStateReq <- req:
				req.peer.FetchNodeData(req.items)
			case <-s.cancel:
			case <-s.d.cancelCh:
			}
		}
	}
}

//filltasks用最多n个状态下载来填充给定的请求对象
//要发送到远程对等机的任务。
func (s *stateSync) fillTasks(n int, req *stateReq) {
//从调度程序重新填充可用任务。
	if len(s.tasks) < n {
		new := s.sched.Missing(n - len(s.tasks))
		for _, hash := range new {
			s.tasks[hash] = &stateTask{make(map[string]struct{})}
		}
	}
//查找尚未使用请求的对等方尝试的任务。
	req.items = make([]common.Hash, 0, n)
	req.tasks = make(map[common.Hash]*stateTask, n)
	for hash, t := range s.tasks {
//当我们收集到足够多的请求时停止
		if len(req.items) == n {
			break
		}
//跳过我们已经尝试过的来自此对等方的任何请求
		if _, ok := t.attempts[req.peer.id]; ok {
			continue
		}
//将请求分配给该对等方
		t.attempts[req.peer.id] = struct{}{}
		req.items = append(req.items, hash)
		req.tasks[hash] = t
		delete(s.tasks, hash)
	}
}

//进程迭代一批已交付状态数据，并注入每个项
//进入运行状态同步，重新排队请求但没有的任何项目
//交付。返回对等端是否实际成功地传递了
//值，以及发生的任何错误。
func (s *stateSync) process(req *stateReq) (int, error) {
//Collect processing stats and update progress if valid data was received
	duplicate, unexpected, successful := 0, 0, 0

	defer func(start time.Time) {
		if duplicate > 0 || unexpected > 0 {
			s.updateStats(0, duplicate, unexpected, time.Since(start))
		}
	}(time.Now())

//对所有传递的数据进行迭代，并逐个注入到trie中
	for _, blob := range req.response {
		_, hash, err := s.processNodeData(blob)
		switch err {
		case nil:
			s.numUncommitted++
			s.bytesUncommitted += len(blob)
			successful++
		case trie.ErrNotRequested:
			unexpected++
		case trie.ErrAlreadyProcessed:
			duplicate++
		default:
			return successful, fmt.Errorf("invalid state node %s: %v", hash.TerminalString(), err)
		}
		if _, ok := req.tasks[hash]; ok {
			delete(req.tasks, hash)
		}
	}
//将未完成的任务放回重试队列
	npeers := s.d.peers.Len()
	for hash, task := range req.tasks {
//If the node did deliver something, missing items may be due to a protocol
//限制或以前的超时+延迟传递。两种情况都应该允许
//要重试丢失项的节点（以避免单点暂停）。
		if len(req.response) > 0 || req.timedOut() {
			delete(task.attempts, req.peer.id)
		}
//如果我们已经请求节点太多次，可能是恶意的
//在没有人拥有正确数据的地方同步。中止。
		if len(task.attempts) >= npeers {
			return successful, fmt.Errorf("state node %s failed with all peers (%d tries, %d peers)", hash.TerminalString(), len(task.attempts), npeers)
		}
//缺少项，请放入重试队列。
		s.tasks[hash] = task
	}
	return successful, nil
}

//processNodeData尝试插入从远程服务器传递的trie节点数据blob
//查看状态trie，返回是否编写了有用的内容或
//发生错误。
func (s *stateSync) processNodeData(blob []byte) (bool, common.Hash, error) {
	res := trie.SyncResult{Data: blob}
	s.keccak.Reset()
	s.keccak.Write(blob)
	s.keccak.Sum(res.Hash[:0])
	committed, _, err := s.sched.Process([]trie.SyncResult{res})
	return committed, res.Hash, err
}

//updateStats触发各种状态同步进度计数器并显示日志
//供用户查看的消息。
func (s *stateSync) updateStats(written, duplicate, unexpected int, duration time.Duration) {
	s.d.syncStatsLock.Lock()
	defer s.d.syncStatsLock.Unlock()

	s.d.syncStatsState.pending = uint64(s.sched.Pending())
	s.d.syncStatsState.processed += uint64(written)
	s.d.syncStatsState.duplicate += uint64(duplicate)
	s.d.syncStatsState.unexpected += uint64(unexpected)

	if written > 0 || duplicate > 0 || unexpected > 0 {
		log.Info("Imported new state entries", "count", written, "elapsed", common.PrettyDuration(duration), "processed", s.d.syncStatsState.processed, "pending", s.d.syncStatsState.pending, "retry", len(s.tasks), "duplicate", s.d.syncStatsState.duplicate, "unexpected", s.d.syncStatsState.unexpected)
	}
	if written > 0 {
		rawdb.WriteFastTrieProgress(s.d.stateDB, s.d.syncStatsState.processed)
	}
}
