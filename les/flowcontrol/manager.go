
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

//包流控制实现客户端流控制机制
package flowcontrol

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/mclock"
)

const rcConst = 1000000

type cmNode struct {
	node                         *ClientNode
	lastUpdate                   mclock.AbsTime
	serving, recharging          bool
	rcWeight                     uint64
	rcValue, rcDelta, startValue int64
	finishRecharge               mclock.AbsTime
}

func (node *cmNode) update(time mclock.AbsTime) {
	dt := int64(time - node.lastUpdate)
	node.rcValue += node.rcDelta * dt / rcConst
	node.lastUpdate = time
	if node.recharging && time >= node.finishRecharge {
		node.recharging = false
		node.rcDelta = 0
		node.rcValue = 0
	}
}

func (node *cmNode) set(serving bool, simReqCnt, sumWeight uint64) {
	if node.serving && !serving {
		node.recharging = true
		sumWeight += node.rcWeight
	}
	node.serving = serving
	if node.recharging && serving {
		node.recharging = false
		sumWeight -= node.rcWeight
	}

	node.rcDelta = 0
	if serving {
		node.rcDelta = int64(rcConst / simReqCnt)
	}
	if node.recharging {
		node.rcDelta = -int64(node.node.cm.rcRecharge * node.rcWeight / sumWeight)
		node.finishRecharge = node.lastUpdate + mclock.AbsTime(node.rcValue*rcConst/(-node.rcDelta))
	}
}

type ClientManager struct {
	lock                             sync.Mutex
	nodes                            map[*cmNode]struct{}
	simReqCnt, sumWeight, rcSumValue uint64
	maxSimReq, maxRcSum              uint64
	rcRecharge                       uint64
	resumeQueue                      chan chan bool
	time                             mclock.AbsTime
}

func NewClientManager(rcTarget, maxSimReq, maxRcSum uint64) *ClientManager {
	cm := &ClientManager{
		nodes:       make(map[*cmNode]struct{}),
		resumeQueue: make(chan chan bool),
		rcRecharge:  rcConst * rcConst / (100*rcConst/rcTarget - rcConst),
		maxSimReq:   maxSimReq,
		maxRcSum:    maxRcSum,
	}
	go cm.queueProc()
	return cm
}

func (self *ClientManager) Stop() {
	self.lock.Lock()
	defer self.lock.Unlock()

//向任何等待接受例程发出返回false的信号
	self.nodes = make(map[*cmNode]struct{})
	close(self.resumeQueue)
}

func (self *ClientManager) addNode(cnode *ClientNode) *cmNode {
	time := mclock.Now()
	node := &cmNode{
		node:           cnode,
		lastUpdate:     time,
		finishRecharge: time,
		rcWeight:       1,
	}
	self.lock.Lock()
	defer self.lock.Unlock()

	self.nodes[node] = struct{}{}
	self.update(mclock.Now())
	return node
}

func (self *ClientManager) removeNode(node *cmNode) {
	self.lock.Lock()
	defer self.lock.Unlock()

	time := mclock.Now()
	self.stop(node, time)
	delete(self.nodes, node)
	self.update(time)
}

//重新计算总重量
func (self *ClientManager) updateNodes(time mclock.AbsTime) (rce bool) {
	var sumWeight, rcSum uint64
	for node := range self.nodes {
		rc := node.recharging
		node.update(time)
		if rc && !node.recharging {
			rce = true
		}
		if node.recharging {
			sumWeight += node.rcWeight
		}
		rcSum += uint64(node.rcValue)
	}
	self.sumWeight = sumWeight
	self.rcSumValue = rcSum
	return
}

func (self *ClientManager) update(time mclock.AbsTime) {
	for {
		firstTime := time
		for node := range self.nodes {
			if node.recharging && node.finishRecharge < firstTime {
				firstTime = node.finishRecharge
			}
		}
		if self.updateNodes(firstTime) {
			for node := range self.nodes {
				if node.recharging {
					node.set(node.serving, self.simReqCnt, self.sumWeight)
				}
			}
		} else {
			self.time = time
			return
		}
	}
}

func (self *ClientManager) canStartReq() bool {
	return self.simReqCnt < self.maxSimReq && self.rcSumValue < self.maxRcSum
}

func (self *ClientManager) queueProc() {
	for rc := range self.resumeQueue {
		for {
			time.Sleep(time.Millisecond * 10)
			self.lock.Lock()
			self.update(mclock.Now())
			cs := self.canStartReq()
			self.lock.Unlock()
			if cs {
				break
			}
		}
		close(rc)
	}
}

func (self *ClientManager) accept(node *cmNode, time mclock.AbsTime) bool {
	self.lock.Lock()
	defer self.lock.Unlock()

	self.update(time)
	if !self.canStartReq() {
		resume := make(chan bool)
		self.lock.Unlock()
		self.resumeQueue <- resume
		<-resume
		self.lock.Lock()
		if _, ok := self.nodes[node]; !ok {
return false //如果节点已删除或管理器已停止，则拒绝
		}
	}
	self.simReqCnt++
	node.set(true, self.simReqCnt, self.sumWeight)
	node.startValue = node.rcValue
	self.update(self.time)
	return true
}

func (self *ClientManager) stop(node *cmNode, time mclock.AbsTime) {
	if node.serving {
		self.update(time)
		self.simReqCnt--
		node.set(false, self.simReqCnt, self.sumWeight)
		self.update(time)
	}
}

func (self *ClientManager) processed(node *cmNode, time mclock.AbsTime) (rcValue, rcCost uint64) {
	self.lock.Lock()
	defer self.lock.Unlock()

	self.stop(node, time)
	return uint64(node.rcValue), uint64(node.rcValue - node.startValue)
}
