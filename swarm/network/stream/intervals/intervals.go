
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

package intervals

import (
	"bytes"
	"fmt"
	"strconv"
	"sync"
)

//间隔存储间隔列表。其目的是提供
//方法添加新间隔并检索
//需要添加。
//它可以用于流数据的同步以保持
//已检索到会话之间的数据范围。
type Intervals struct {
	start  uint64
	ranges [][2]uint64
	mu     sync.RWMutex
}

//新建创建间隔的新实例。
//start参数限制间隔的下限。
//添加方法或将不添加以下开始绑定的范围
//由下一个方法返回。此限制可用于
//跟踪“实时”同步，其中同步会话
//从特定值开始，如果“实时”同步间隔
//需要与历史相结合，才能安全地完成。
func NewIntervals(start uint64) *Intervals {
	return &Intervals{
		start: start,
	}
}

//添加将新范围添加到间隔。范围开始和结束都是值
//都是包容性的。
func (i *Intervals) Add(start, end uint64) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.add(start, end)
}

func (i *Intervals) add(start, end uint64) {
	if start < i.start {
		start = i.start
	}
	if end < i.start {
		return
	}
	minStartJ := -1
	maxEndJ := -1
	j := 0
	for ; j < len(i.ranges); j++ {
		if minStartJ < 0 {
			if (start <= i.ranges[j][0] && end+1 >= i.ranges[j][0]) || (start <= i.ranges[j][1]+1 && end+1 >= i.ranges[j][1]) {
				if i.ranges[j][0] < start {
					start = i.ranges[j][0]
				}
				minStartJ = j
			}
		}
		if (start <= i.ranges[j][1] && end+1 >= i.ranges[j][1]) || (start <= i.ranges[j][0] && end+1 >= i.ranges[j][0]) {
			if i.ranges[j][1] > end {
				end = i.ranges[j][1]
			}
			maxEndJ = j
		}
		if end+1 <= i.ranges[j][0] {
			break
		}
	}
	if minStartJ < 0 && maxEndJ < 0 {
		i.ranges = append(i.ranges[:j], append([][2]uint64{{start, end}}, i.ranges[j:]...)...)
		return
	}
	if minStartJ >= 0 {
		i.ranges[minStartJ][0] = start
	}
	if maxEndJ >= 0 {
		i.ranges[maxEndJ][1] = end
	}
	if minStartJ >= 0 && maxEndJ >= 0 && minStartJ != maxEndJ {
		i.ranges[maxEndJ][0] = start
		i.ranges = append(i.ranges[:minStartJ], i.ranges[maxEndJ:]...)
	}
}

//合并将m间隔中的所有间隔添加到当前间隔。
func (i *Intervals) Merge(m *Intervals) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	i.mu.Lock()
	defer i.mu.Unlock()

	for _, r := range m.ranges {
		i.add(r[0], r[1])
	}
}

//Next返回未完成的第一个范围间隔。返回
//起始值和结束值都包含在内，这意味着整个范围
//包括开始和结束需要增加，以填补差距
//每隔一段时间。
//如果下一个间隔在整数之后，则end的返回值为0
//以间隔存储的范围。零结束值表示无限制
//在下一个间隔长度。
func (i *Intervals) Next() (start, end uint64) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	l := len(i.ranges)
	if l == 0 {
		return i.start, 0
	}
	if i.ranges[0][0] != i.start {
		return i.start, i.ranges[0][0] - 1
	}
	if l == 1 {
		return i.ranges[0][1] + 1, 0
	}
	return i.ranges[0][1] + 1, i.ranges[1][0] - 1
}

//Last返回最后一个间隔结束时的值。
func (i *Intervals) Last() (end uint64) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	l := len(i.ranges)
	if l == 0 {
		return 0
	}
	return i.ranges[l-1][1]
}

//字符串返回范围间隔的描述性表示形式
//以[]表示法，作为两个元素向量的列表。
func (i *Intervals) String() string {
	return fmt.Sprint(i.ranges)
}

//marshalbinary将间隔参数编码为分号分隔列表。
//列表中的第一个元素是base36编码的起始值。以下
//元素是由逗号分隔的两个base36编码值范围。
func (i *Intervals) MarshalBinary() (data []byte, err error) {
	d := make([][]byte, len(i.ranges)+1)
	d[0] = []byte(strconv.FormatUint(i.start, 36))
	for j := range i.ranges {
		r := i.ranges[j]
		d[j+1] = []byte(strconv.FormatUint(r[0], 36) + "," + strconv.FormatUint(r[1], 36))
	}
	return bytes.Join(d, []byte(";")), nil
}

//unmarshalbinary根据interval.marshalbinary格式解码数据。
func (i *Intervals) UnmarshalBinary(data []byte) (err error) {
	d := bytes.Split(data, []byte(";"))
	l := len(d)
	if l == 0 {
		return nil
	}
	if l >= 1 {
		i.start, err = strconv.ParseUint(string(d[0]), 36, 64)
		if err != nil {
			return err
		}
	}
	if l == 1 {
		return nil
	}

	i.ranges = make([][2]uint64, 0, l-1)
	for j := 1; j < l; j++ {
		r := bytes.SplitN(d[j], []byte(","), 2)
		if len(r) < 2 {
			return fmt.Errorf("range %d has less then 2 elements", j)
		}
		start, err := strconv.ParseUint(string(r[0]), 36, 64)
		if err != nil {
			return fmt.Errorf("parsing the first element in range %d: %v", j, err)
		}
		end, err := strconv.ParseUint(string(r[1]), 36, 64)
		if err != nil {
			return fmt.Errorf("parsing the second element in range %d: %v", j, err)
		}
		i.ranges = append(i.ranges, [2]uint64{start, end})
	}

	return nil
}
