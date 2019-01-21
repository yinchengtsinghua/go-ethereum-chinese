
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//这是“gopkg.in/karalabe/cookiejar.v2/collections/prque”的一个复制和稍加修改的版本。

package prque

import (
	"container/heap"
)

//优先级队列数据结构。
type Prque struct {
	cont *sstack
}

//创建新的优先级队列。
func New(setIndex setIndexCallback) *Prque {
	return &Prque{newSstack(setIndex)}
}

//将具有给定优先级的值推入队列，必要时展开。
func (p *Prque) Push(data interface{}, priority int64) {
	heap.Push(p.cont, &item{data, priority})
}

//从堆栈中弹出优先级为greates的值并返回该值。
//目前还没有收缩。
func (p *Prque) Pop() (interface{}, int64) {
	item := heap.Pop(p.cont).(*item)
	return item.value, item.priority
}

//只从队列中弹出项目，删除关联的优先级值。
func (p *Prque) PopItem() interface{} {
	return heap.Pop(p.cont).(*item).value
}

//移除移除具有给定索引的元素。
func (p *Prque) Remove(i int) interface{} {
	if i < 0 {
		return nil
	}
	return heap.Remove(p.cont, i)
}

//检查优先级队列是否为空。
func (p *Prque) Empty() bool {
	return p.cont.Len() == 0
}

//返回优先级队列中的元素数。
func (p *Prque) Size() int {
	return p.cont.Len()
}

//清除优先级队列的内容。
func (p *Prque) Reset() {
	*p = *New(p.cont.setIndex)
}
