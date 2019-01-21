
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//这是“gopkg.in/karalabe/cookiejar.v2/collections/prque”的一个复制和稍加修改的版本。

package prque

//数据块的大小
const blockSize = 4096

//排序堆栈中的优先项。
//
//注意：优先级可以“环绕”Int64范围，如果（a.priority-b.priority）>0，则A在B之前。
//队列中任何点的最低优先级和最高优先级之间的差异应小于2^63。
type item struct {
	value    interface{}
	priority int64
}

//将元素移动到新索引时调用setindexcallback。
//提供setindexcallback是可选的，只有在应用程序需要时才需要它
//删除顶部元素以外的元素。
type setIndexCallback func(a interface{}, i int)

//内部可排序堆栈数据结构。为执行推送和弹出操作
//堆栈（堆）功能和len、less和swap方法
//堆的可分类性要求。
type sstack struct {
	setIndex setIndexCallback
	size     int
	capacity int
	offset   int

	blocks [][]*item
	active []*item
}

//创建新的空堆栈。
func newSstack(setIndex setIndexCallback) *sstack {
	result := new(sstack)
	result.setIndex = setIndex
	result.active = make([]*item, blockSize)
	result.blocks = [][]*item{result.active}
	result.capacity = blockSize
	return result
}

//将值推送到堆栈上，必要时将其展开。要求
//HEAP.接口
func (s *sstack) Push(data interface{}) {
	if s.size == s.capacity {
		s.active = make([]*item, blockSize)
		s.blocks = append(s.blocks, s.active)
		s.capacity += blockSize
		s.offset = 0
	} else if s.offset == blockSize {
		s.active = s.blocks[s.size/blockSize]
		s.offset = 0
	}
	if s.setIndex != nil {
		s.setIndex(data.(*item).value, s.size)
	}
	s.active[s.offset] = data.(*item)
	s.offset++
	s.size++
}

//从堆栈中弹出一个值并返回它。目前还没有收缩。
//heap.interface需要。
func (s *sstack) Pop() (res interface{}) {
	s.size--
	s.offset--
	if s.offset < 0 {
		s.offset = blockSize - 1
		s.active = s.blocks[s.size/blockSize]
	}
	res, s.active[s.offset] = s.active[s.offset], nil
	if s.setIndex != nil {
		s.setIndex(res.(*item).value, -1)
	}
	return
}

//返回堆栈的长度。sort.interface需要。
func (s *sstack) Len() int {
	return s.size
}

//比较堆栈中两个元素的优先级（较高的是第一个）。
//sort.interface需要。
func (s *sstack) Less(i, j int) bool {
	return (s.blocks[i/blockSize][i%blockSize].priority - s.blocks[j/blockSize][j%blockSize].priority) > 0
}

//交换堆栈中的两个元素。sort.interface需要。
func (s *sstack) Swap(i, j int) {
	ib, io, jb, jo := i/blockSize, i%blockSize, j/blockSize, j%blockSize
	a, b := s.blocks[jb][jo], s.blocks[ib][io]
	if s.setIndex != nil {
		s.setIndex(a.value, i)
		s.setIndex(b.value, j)
	}
	s.blocks[ib][io], s.blocks[jb][jo] = a, b
}

//重置堆栈，有效地清除其内容。
func (s *sstack) Reset() {
	*s = *newSstack(s.setIndex)
}
