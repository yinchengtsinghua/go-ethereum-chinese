
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
	"container/heap"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/log"
)

const (
	maxEntries         = 10000
	maxEntriesPerTopic = 50

	fallbackRegistrationExpiry = 1 * time.Hour
)

type Topic string

type topicEntry struct {
	topic   Topic
	fifoIdx uint64
	node    *Node
	expire  mclock.AbsTime
}

type topicInfo struct {
	entries            map[uint64]*topicEntry
	fifoHead, fifoTail uint64
	rqItem             *topicRequestQueueItem
	wcl                waitControlLoop
}

//从FIFO中删除尾部元素
func (t *topicInfo) getFifoTail() *topicEntry {
	for t.entries[t.fifoTail] == nil {
		t.fifoTail++
	}
	tail := t.entries[t.fifoTail]
	t.fifoTail++
	return tail
}

type nodeInfo struct {
	entries                          map[Topic]*topicEntry
	lastIssuedTicket, lastUsedTicket uint32
//您不能在noreguntil（绝对时间）之前注册比lastusedticket更新的票据。
	noRegUntil mclock.AbsTime
}

type topicTable struct {
	db                    *nodeDB
	self                  *Node
	nodes                 map[*Node]*nodeInfo
	topics                map[Topic]*topicInfo
	globalEntries         uint64
	requested             topicRequestQueue
	requestCnt            uint64
	lastGarbageCollection mclock.AbsTime
}

func newTopicTable(db *nodeDB, self *Node) *topicTable {
	if printTestImgLogs {
		fmt.Printf("*N %016x\n", self.sha[:8])
	}
	return &topicTable{
		db:     db,
		nodes:  make(map[*Node]*nodeInfo),
		topics: make(map[Topic]*topicInfo),
		self:   self,
	}
}

func (t *topicTable) getOrNewTopic(topic Topic) *topicInfo {
	ti := t.topics[topic]
	if ti == nil {
		rqItem := &topicRequestQueueItem{
			topic:    topic,
			priority: t.requestCnt,
		}
		ti = &topicInfo{
			entries: make(map[uint64]*topicEntry),
			rqItem:  rqItem,
		}
		t.topics[topic] = ti
		heap.Push(&t.requested, rqItem)
	}
	return ti
}

func (t *topicTable) checkDeleteTopic(topic Topic) {
	ti := t.topics[topic]
	if ti == nil {
		return
	}
	if len(ti.entries) == 0 && ti.wcl.hasMinimumWaitPeriod() {
		delete(t.topics, topic)
		heap.Remove(&t.requested, ti.rqItem.index)
	}
}

func (t *topicTable) getOrNewNode(node *Node) *nodeInfo {
	n := t.nodes[node]
	if n == nil {
//fmt.printf（“newnode%016x%016x\n”，t.self.sha[：8]，node.sha[：8]）
		var issued, used uint32
		if t.db != nil {
			issued, used = t.db.fetchTopicRegTickets(node.ID)
		}
		n = &nodeInfo{
			entries:          make(map[Topic]*topicEntry),
			lastIssuedTicket: issued,
			lastUsedTicket:   used,
		}
		t.nodes[node] = n
	}
	return n
}

func (t *topicTable) checkDeleteNode(node *Node) {
	if n, ok := t.nodes[node]; ok && len(n.entries) == 0 && n.noRegUntil < mclock.Now() {
//fmt.printf（“删除节点%016x%016x\n”，t.self.sha[：8]，节点.sha[：8]）
		delete(t.nodes, node)
	}
}

func (t *topicTable) storeTicketCounters(node *Node) {
	n := t.getOrNewNode(node)
	if t.db != nil {
		t.db.updateTopicRegTickets(node.ID, n.lastIssuedTicket, n.lastUsedTicket)
	}
}

func (t *topicTable) getEntries(topic Topic) []*Node {
	t.collectGarbage()

	te := t.topics[topic]
	if te == nil {
		return nil
	}
	nodes := make([]*Node, len(te.entries))
	i := 0
	for _, e := range te.entries {
		nodes[i] = e.node
		i++
	}
	t.requestCnt++
	t.requested.update(te.rqItem, t.requestCnt)
	return nodes
}

func (t *topicTable) addEntry(node *Node, topic Topic) {
	n := t.getOrNewNode(node)
//按同一节点清除以前的条目
	for _, e := range n.entries {
		t.deleteEntry(e)
	}
//***
	n = t.getOrNewNode(node)

	tm := mclock.Now()
	te := t.getOrNewTopic(topic)

	if len(te.entries) == maxEntriesPerTopic {
		t.deleteEntry(te.getFifoTail())
	}

	if t.globalEntries == maxEntries {
t.deleteEntry(t.leastRequested()) //不是空的，不需要检查零
	}

	fifoIdx := te.fifoHead
	te.fifoHead++
	entry := &topicEntry{
		topic:   topic,
		fifoIdx: fifoIdx,
		node:    node,
		expire:  tm + mclock.AbsTime(fallbackRegistrationExpiry),
	}
	if printTestImgLogs {
		fmt.Printf("*+ %d %v %016x %016x\n", tm/1000000, topic, t.self.sha[:8], node.sha[:8])
	}
	te.entries[fifoIdx] = entry
	n.entries[topic] = entry
	t.globalEntries++
	te.wcl.registered(tm)
}

//从FIFO中删除请求最少的元素
func (t *topicTable) leastRequested() *topicEntry {
	for t.requested.Len() > 0 && t.topics[t.requested[0].topic] == nil {
		heap.Pop(&t.requested)
	}
	if t.requested.Len() == 0 {
		return nil
	}
	return t.topics[t.requested[0].topic].getFifoTail()
}

//条目应该存在
func (t *topicTable) deleteEntry(e *topicEntry) {
	if printTestImgLogs {
		fmt.Printf("*- %d %v %016x %016x\n", mclock.Now()/1000000, e.topic, t.self.sha[:8], e.node.sha[:8])
	}
	ne := t.nodes[e.node].entries
	delete(ne, e.topic)
	if len(ne) == 0 {
		t.checkDeleteNode(e.node)
	}
	te := t.topics[e.topic]
	delete(te.entries, e.fifoIdx)
	if len(te.entries) == 0 {
		t.checkDeleteTopic(e.topic)
	}
	t.globalEntries--
}

//假设主题和等待期的长度相同。
func (t *topicTable) useTicket(node *Node, serialNo uint32, topics []Topic, idx int, issueTime uint64, waitPeriods []uint32) (registered bool) {
	log.Trace("Using discovery ticket", "serial", serialNo, "topics", topics, "waits", waitPeriods)
//fmt.println（“useticket”，serialno，topics，waitperiods）
	t.collectGarbage()

	n := t.getOrNewNode(node)
	if serialNo < n.lastUsedTicket {
		return false
	}

	tm := mclock.Now()
	if serialNo > n.lastUsedTicket && tm < n.noRegUntil {
		return false
	}
	if serialNo != n.lastUsedTicket {
		n.lastUsedTicket = serialNo
		n.noRegUntil = tm + mclock.AbsTime(noRegTimeout())
		t.storeTicketCounters(node)
	}

	currTime := uint64(tm / mclock.AbsTime(time.Second))
	regTime := issueTime + uint64(waitPeriods[idx])
	relTime := int64(currTime - regTime)
if relTime >= -1 && relTime <= regTimeWindow+1 { //在两端给客户一点安全边际
		if e := n.entries[topics[idx]]; e == nil {
			t.addEntry(node, topics[idx])
		} else {
//如果有活动条目，不要移动到FIFO的前面，而是延长过期时间
			e.expire = tm + mclock.AbsTime(fallbackRegistrationExpiry)
		}
		return true
	}

	return false
}

func (t *topicTable) getTicket(node *Node, topics []Topic) *ticket {
	t.collectGarbage()

	now := mclock.Now()
	n := t.getOrNewNode(node)
	n.lastIssuedTicket++
	t.storeTicketCounters(node)

	tic := &ticket{
		issueTime: now,
		topics:    topics,
		serial:    n.lastIssuedTicket,
		regTime:   make([]mclock.AbsTime, len(topics)),
	}
	for i, topic := range topics {
		var waitPeriod time.Duration
		if topic := t.topics[topic]; topic != nil {
			waitPeriod = topic.wcl.waitPeriod
		} else {
			waitPeriod = minWaitPeriod
		}

		tic.regTime[i] = now + mclock.AbsTime(waitPeriod)
	}
	return tic
}

const gcInterval = time.Minute

func (t *topicTable) collectGarbage() {
	tm := mclock.Now()
	if time.Duration(tm-t.lastGarbageCollection) < gcInterval {
		return
	}
	t.lastGarbageCollection = tm

	for node, n := range t.nodes {
		for _, e := range n.entries {
			if e.expire <= tm {
				t.deleteEntry(e)
			}
		}

		t.checkDeleteNode(node)
	}

	for topic := range t.topics {
		t.checkDeleteTopic(topic)
	}
}

const (
	minWaitPeriod   = time.Minute
regTimeWindow   = 10 //秒
	avgnoRegTimeout = time.Minute * 10
//两个传入AD请求之间的目标平均间隔
	wcTargetRegInterval = time.Minute * 10 / maxEntriesPerTopic
//
	wcTimeConst = time.Minute * 10
)

//不需要初始化，将在首次注册时设置为minwaitperiod
type waitControlLoop struct {
	lastIncoming mclock.AbsTime
	waitPeriod   time.Duration
}

func (w *waitControlLoop) registered(tm mclock.AbsTime) {
	w.waitPeriod = w.nextWaitPeriod(tm)
	w.lastIncoming = tm
}

func (w *waitControlLoop) nextWaitPeriod(tm mclock.AbsTime) time.Duration {
	period := tm - w.lastIncoming
	wp := time.Duration(float64(w.waitPeriod) * math.Exp((float64(wcTargetRegInterval)-float64(period))/float64(wcTimeConst)))
	if wp < minWaitPeriod {
		wp = minWaitPeriod
	}
	return wp
}

func (w *waitControlLoop) hasMinimumWaitPeriod() bool {
	return w.nextWaitPeriod(mclock.Now()) == minWaitPeriod
}

func noRegTimeout() time.Duration {
	e := rand.ExpFloat64()
	if e > 100 {
		e = 100
	}
	return time.Duration(float64(avgnoRegTimeout) * e)
}

type topicRequestQueueItem struct {
	topic    Topic
	priority uint64
	index    int
}

//TopicRequestQueue实现heap.interface并保存TopicRequestQueueitems。
type topicRequestQueue []*topicRequestQueueItem

func (tq topicRequestQueue) Len() int { return len(tq) }

func (tq topicRequestQueue) Less(i, j int) bool {
	return tq[i].priority < tq[j].priority
}

func (tq topicRequestQueue) Swap(i, j int) {
	tq[i], tq[j] = tq[j], tq[i]
	tq[i].index = i
	tq[j].index = j
}

func (tq *topicRequestQueue) Push(x interface{}) {
	n := len(*tq)
	item := x.(*topicRequestQueueItem)
	item.index = n
	*tq = append(*tq, item)
}

func (tq *topicRequestQueue) Pop() interface{} {
	old := *tq
	n := len(old)
	item := old[n-1]
	item.index = -1
	*tq = old[0 : n-1]
	return item
}

func (tq *topicRequestQueue) update(item *topicRequestQueueItem, priority uint64) {
	item.priority = priority
	heap.Fix(tq, item.index)
}
