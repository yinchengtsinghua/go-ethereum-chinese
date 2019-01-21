
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

//包装罐见Go医生
package pot

import (
	"fmt"
	"sync"
)

const (
	maxkeylen = 256
)

//pot是节点类型（根、分支节点和叶相同）
type Pot struct {
	pin  Val
	bins []*Pot
	size int
	po   int
}

//VAL是锅的元件类型
type Val interface{}

//pof是邻近订单比较运算符函数
type Pof func(Val, Val, int) (int, bool)

//Newpot构造函数。需要VAL类型的值才能固定
//和po指向val键中的跨度
//钉住的项目按大小计算
func NewPot(v Val, po int) *Pot {
	var size int
	if v != nil {
		size++
	}
	return &Pot{
		pin:  v,
		po:   po,
		size: size,
	}
}

//pin返回pot的pinned元素（键）
func (t *Pot) Pin() Val {
	return t.pin
}

//SIZE返回pot中的值数
func (t *Pot) Size() int {
	if t == nil {
		return 0
	}
	return t.size
}

//添加将新值插入到容器中，然后
//返回v和布尔值的接近顺序
//指示是否找到该项
//add called on（t，v）返回包含t的所有元素的新pot
//加上V值，使用应用加法
//第二个返回值是插入元素的接近顺序。
//第三个是布尔值，指示是否找到该项
func Add(t *Pot, val Val, pof Pof) (*Pot, int, bool) {
	return add(t, val, pof)
}

func (t *Pot) clone() *Pot {
	return &Pot{
		pin:  t.pin,
		size: t.size,
		po:   t.po,
		bins: t.bins,
	}
}

func add(t *Pot, val Val, pof Pof) (*Pot, int, bool) {
	var r *Pot
	if t == nil || t.pin == nil {
		r = t.clone()
		r.pin = val
		r.size++
		return r, 0, false
	}
	po, found := pof(t.pin, val, t.po)
	if found {
		r = t.clone()
		r.pin = val
		return r, po, true
	}

	var p *Pot
	var i, j int
	size := t.size
	for i < len(t.bins) {
		n := t.bins[i]
		if n.po == po {
			p, _, found = add(n, val, pof)
			if !found {
				size++
			}
			j++
			break
		}
		if n.po > po {
			break
		}
		i++
		j++
	}
	if p == nil {
		size++
		p = &Pot{
			pin:  val,
			size: 1,
			po:   po,
		}
	}

	bins := append([]*Pot{}, t.bins[:i]...)
	bins = append(bins, p)
	bins = append(bins, t.bins[j:]...)
	r = &Pot{
		pin:  t.pin,
		size: size,
		po:   t.po,
		bins: bins,
	}

	return r, po, found
}

//remove从pot t t中删除元素v并返回三个参数：
//1。新锅，包含所有元素t减去元素v；
//2。拆除元件V的接近顺序；
//三。指示是否找到该项的布尔值。
func Remove(t *Pot, v Val, pof Pof) (*Pot, int, bool) {
	return remove(t, v, pof)
}

func remove(t *Pot, val Val, pof Pof) (r *Pot, po int, found bool) {
	size := t.size
	po, found = pof(t.pin, val, t.po)
	if found {
		size--
		if size == 0 {
			return &Pot{}, po, true
		}
		i := len(t.bins) - 1
		last := t.bins[i]
		r = &Pot{
			pin:  last.pin,
			bins: append(t.bins[:i], last.bins...),
			size: size,
			po:   t.po,
		}
		return r, t.po, true
	}

	var p *Pot
	var i, j int
	for i < len(t.bins) {
		n := t.bins[i]
		if n.po == po {
			p, po, found = remove(n, val, pof)
			if found {
				size--
			}
			j++
			break
		}
		if n.po > po {
			return t, po, false
		}
		i++
		j++
	}
	bins := t.bins[:i]
	if p != nil && p.pin != nil {
		bins = append(bins, p)
	}
	bins = append(bins, t.bins[j:]...)
	r = &Pot{
		pin:  t.pin,
		size: size,
		po:   t.po,
		bins: bins,
	}
	return r, po, found
}

//调用的swap（k，f）在k处查找项
//
//如果f（v）返回nil，则删除元素
//如果f（v）返回v'<>v，则v'插入锅中。
//如果（v）==v，则罐不更换。
//如果pof（f（v），k）显示v'和v不是键相等，则会恐慌。
func Swap(t *Pot, k Val, pof Pof, f func(v Val) Val) (r *Pot, po int, found bool, change bool) {
	var val Val
	if t.pin == nil {
		val = f(nil)
		if val == nil {
			return nil, 0, false, false
		}
		return NewPot(val, t.po), 0, false, true
	}
	size := t.size
	po, found = pof(k, t.pin, t.po)
	if found {
		val = f(t.pin)
//移除元素
		if val == nil {
			size--
			if size == 0 {
				r = &Pot{
					po: t.po,
				}
//返回空罐
				return r, po, true, true
			}
//实际上，通过合并最后一个bin来删除pin
			i := len(t.bins) - 1
			last := t.bins[i]
			r = &Pot{
				pin:  last.pin,
				bins: append(t.bins[:i], last.bins...),
				size: size,
				po:   t.po,
			}
			return r, po, true, true
		}
//找到元素，但没有更改
		if val == t.pin {
			return t, po, true, false
		}
//实际修改固定元素，但结构没有更改
		r = t.clone()
		r.pin = val
		return r, po, true, true
	}

//递归阶段
	var p *Pot
	n, i := t.getPos(po)
	if n != nil {
		p, po, found, change = Swap(n, k, pof, f)
//递归无更改
		if !change {
			return t, po, found, false
		}
//递归更改
		bins := append([]*Pot{}, t.bins[:i]...)
		if p.size == 0 {
			size--
		} else {
			size += p.size - n.size
			bins = append(bins, p)
		}
		i++
		if i < len(t.bins) {
			bins = append(bins, t.bins[i:]...)
		}
		r = t.clone()
		r.bins = bins
		r.size = size
		return r, po, found, true
	}
//密钥不存在
	val = f(nil)
	if val == nil {
//它不应该被创建
		return t, po, false, false
	}
//否则，如果等于k，则检查val
	if _, eq := pof(val, k, po); !eq {
		panic("invalid value")
	}
///
	size++
	p = &Pot{
		pin:  val,
		size: 1,
		po:   po,
	}

	bins := append([]*Pot{}, t.bins[:i]...)
	bins = append(bins, p)
	if i < len(t.bins) {
		bins = append(bins, t.bins[i:]...)
	}
	r = t.clone()
	r.bins = bins
	r.size = size
	return r, po, found, true
}

//在（t0，t1，pof）上调用的union返回t0和t1的union
//使用应用联合计算联合
//第二个返回值是公共元素的数目
func Union(t0, t1 *Pot, pof Pof) (*Pot, int) {
	return union(t0, t1, pof)
}

func union(t0, t1 *Pot, pof Pof) (*Pot, int) {
	if t0 == nil || t0.size == 0 {
		return t1, 0
	}
	if t1 == nil || t1.size == 0 {
		return t0, 0
	}
	var pin Val
	var bins []*Pot
	var mis []int
	wg := &sync.WaitGroup{}
	wg.Add(1)
	pin0 := t0.pin
	pin1 := t1.pin
	bins0 := t0.bins
	bins1 := t1.bins
	var i0, i1 int
	var common int

	po, eq := pof(pin0, pin1, 0)

	for {
		l0 := len(bins0)
		l1 := len(bins1)
		var n0, n1 *Pot
		var p0, p1 int
		var a0, a1 bool

		for {

			if !a0 && i0 < l0 && bins0[i0] != nil && bins0[i0].po <= po {
				n0 = bins0[i0]
				p0 = n0.po
				a0 = p0 == po
			} else {
				a0 = true
			}

			if !a1 && i1 < l1 && bins1[i1] != nil && bins1[i1].po <= po {
				n1 = bins1[i1]
				p1 = n1.po
				a1 = p1 == po
			} else {
				a1 = true
			}
			if a0 && a1 {
				break
			}

			switch {
			case (p0 < p1 || a1) && !a0:
				bins = append(bins, n0)
				i0++
				n0 = nil
			case (p1 < p0 || a0) && !a1:
				bins = append(bins, n1)
				i1++
				n1 = nil
			case p1 < po:
				bl := len(bins)
				bins = append(bins, nil)
				ml := len(mis)
				mis = append(mis, 0)
//添加（1）
//go func（b，m int，m0，m1*pot）
//推迟WG.DONE（）
//箱[B]，MIS[M]=联管节（M0、M1、POF）
//（bl，ml，n0，n1）
				bins[bl], mis[ml] = union(n0, n1, pof)
				i0++
				i1++
				n0 = nil
				n1 = nil
			}
		}

		if eq {
			common++
			pin = pin1
			break
		}

		i := i0
		if len(bins0) > i && bins0[i].po == po {
			i++
		}
		var size0 int
		for _, n := range bins0[i:] {
			size0 += n.size
		}
		np := &Pot{
			pin:  pin0,
			bins: bins0[i:],
			size: size0 + 1,
			po:   po,
		}

		bins2 := []*Pot{np}
		if n0 == nil {
			pin0 = pin1
			po = maxkeylen + 1
			eq = true
			common--

		} else {
			bins2 = append(bins2, n0.bins...)
			pin0 = pin1
			pin1 = n0.pin
			po, eq = pof(pin0, pin1, n0.po)

		}
		bins0 = bins1
		bins1 = bins2
		i0 = i1
		i1 = 0

	}

	wg.Done()
	wg.Wait()
	for _, c := range mis {
		common += c
	}
	n := &Pot{
		pin:  pin,
		bins: bins,
		size: t0.size + t1.size - common,
		po:   t0.po,
	}
	return n, common
}

//每个函数都是pot元素上的同步迭代器，函数为f。
func (t *Pot) Each(f func(Val) bool) bool {
	return t.each(f)
}

//每个函数都是pot元素上的同步迭代器，函数为f。
//如果函数返回false或没有其他元素，则迭代将结束。
func (t *Pot) each(f func(Val) bool) bool {
	if t == nil || t.size == 0 {
		return false
	}
	for _, n := range t.bins {
		if !n.each(f) {
			return false
		}
	}
	return f(t.pin)
}

//eachFrom是对pot元素的同步迭代器，函数为f，
//从某个邻近订单po开始，作为第二个参数传递。
//如果函数返回false或没有其他元素，则迭代将结束。
func (t *Pot) eachFrom(f func(Val) bool, po int) bool {
	if t == nil || t.size == 0 {
		return false
	}
	_, beg := t.getPos(po)
	for i := beg; i < len(t.bins); i++ {
		if !t.bins[i].each(f) {
			return false
		}
	}
	return f(t.pin)
}

//eachbin在pivot节点的bin上迭代，并在每个pivot节点上向调用者提供迭代器。
//传递接近顺序和大小的子树
//迭代将继续，直到函数的返回值为假
//或者没有其他子项
func (t *Pot) EachBin(val Val, pof Pof, po int, f func(int, int, func(func(val Val) bool) bool) bool) {
	t.eachBin(val, pof, po, f)
}

func (t *Pot) eachBin(val Val, pof Pof, po int, f func(int, int, func(func(val Val) bool) bool) bool) {
	if t == nil || t.size == 0 {
		return
	}
	spr, _ := pof(t.pin, val, t.po)
	_, lim := t.getPos(spr)
	var size int
	var n *Pot
	for i := 0; i < lim; i++ {
		n = t.bins[i]
		size += n.size
		if n.po < po {
			continue
		}
		if !f(n.po, n.size, n.each) {
			return
		}
	}
	if lim == len(t.bins) {
		if spr >= po {
			f(spr, 1, func(g func(Val) bool) bool {
				return g(t.pin)
			})
		}
		return
	}

	n = t.bins[lim]

	spo := spr
	if n.po == spr {
		spo++
		size += n.size
	}
	if spr >= po {
		if !f(spr, t.size-size, func(g func(Val) bool) bool {
			return t.eachFrom(func(v Val) bool {
				return g(v)
			}, spo)
		}) {
			return
		}
	}
	if n.po == spr {
		n.eachBin(val, pof, po, f)
	}

}

//eachneighbur是任何目标val的邻居上的同步迭代器。
//元素的检索顺序反映了目标的接近顺序。
//TODO:将最大proxybin添加到迭代的开始范围
func (t *Pot) EachNeighbour(val Val, pof Pof, f func(Val, int) bool) bool {
	return t.eachNeighbour(val, pof, f)
}

func (t *Pot) eachNeighbour(val Val, pof Pof, f func(Val, int) bool) bool {
	if t == nil || t.size == 0 {
		return false
	}
	var next bool
	l := len(t.bins)
	var n *Pot
	ir := l
	il := l
	po, eq := pof(t.pin, val, t.po)
	if !eq {
		n, il = t.getPos(po)
		if n != nil {
			next = n.eachNeighbour(val, pof, f)
			if !next {
				return false
			}
			ir = il
		} else {
			ir = il - 1
		}
	}

	next = f(t.pin, po)
	if !next {
		return false
	}

	for i := l - 1; i > ir; i-- {
		next = t.bins[i].each(func(v Val) bool {
			return f(v, po)
		})
		if !next {
			return false
		}
	}

	for i := il - 1; i >= 0; i-- {
		n := t.bins[i]
		next = n.each(func(v Val) bool {
			return f(v, n.po)
		})
		if !next {
			return false
		}
	}
	return true
}

//调用的eachneighbourasync（val、max、maxpos、f、wait）是异步迭代器
//在不小于maxpos wrt val的元素上。
//val不需要与pot元素匹配，但如果匹配，并且
//maxpos是keylength，而不是包含在迭代中的keylength
//对f的调用是并行的，调用顺序未定义。
//在锅中没有元素
//如果访问了更近的节点，则不会访问。
//当访问最大最近节点数时，迭代完成。
//或者，如果整个系统中没有不比未访问的maxpos更近的节点
//如果wait为true，则仅当对f的所有调用都完成时，迭代器才返回
//TODO:为正确的代理范围迭代实现minpos
func (t *Pot) EachNeighbourAsync(val Val, pof Pof, max int, maxPos int, f func(Val, int), wait bool) {
	if max > t.size {
		max = t.size
	}
	var wg *sync.WaitGroup
	if wait {
		wg = &sync.WaitGroup{}
	}
	t.eachNeighbourAsync(val, pof, max, maxPos, f, wg)
	if wait {
		wg.Wait()
	}
}

func (t *Pot) eachNeighbourAsync(val Val, pof Pof, max int, maxPos int, f func(Val, int), wg *sync.WaitGroup) (extra int) {
	l := len(t.bins)

	po, eq := pof(t.pin, val, t.po)

//如果po太近，将pivot branch（pom）设置为maxpos
	pom := po
	if pom > maxPos {
		pom = maxPos
	}
	n, il := t.getPos(pom)
	ir := il
//如果透视分支存在且采购订单不太接近，则迭代透视分支
	if pom == po {
		if n != nil {

			m := n.size
			if max < m {
				m = max
			}
			max -= m

			extra = n.eachNeighbourAsync(val, pof, m, maxPos, f, wg)

		} else {
			if !eq {
				ir--
			}
		}
	} else {
		extra++
		max--
		if n != nil {
			il++
		}
//在检查max之前，将额外的元素相加
//在被跳过的关闭分支上（如果采购订单太近）
		for i := l - 1; i >= il; i-- {
			s := t.bins[i]
			m := s.size
			if max < m {
				m = max
			}
			max -= m
			extra += m
		}
	}

	var m int
	if pom == po {

		m, max, extra = need(1, max, extra)
		if m <= 0 {
			return
		}

		if wg != nil {
			wg.Add(1)
		}
		go func() {
			if wg != nil {
				defer wg.Done()
			}
			f(t.pin, po)
		}()

//否则迭代
		for i := l - 1; i > ir; i-- {
			n := t.bins[i]

			m, max, extra = need(n.size, max, extra)
			if m <= 0 {
				return
			}

			if wg != nil {
				wg.Add(m)
			}
			go func(pn *Pot, pm int) {
				pn.each(func(v Val) bool {
					if wg != nil {
						defer wg.Done()
					}
					f(v, po)
					pm--
					return pm > 0
				})
			}(n, m)

		}
	}

//使用自己的po迭代更远的tham pom分支
	for i := il - 1; i >= 0; i-- {
		n := t.bins[i]
//第一次max小于整个分支的大小
//等待轴线程释放额外的元素
		m, max, extra = need(n.size, max, extra)
		if m <= 0 {
			return
		}

		if wg != nil {
			wg.Add(m)
		}
		go func(pn *Pot, pm int) {
			pn.each(func(v Val) bool {
				if wg != nil {
					defer wg.Done()
				}
				f(v, pn.po)
				pm--
				return pm > 0
			})
		}(n, m)

	}
	return max + extra
}

//getpos called on（n）返回po n处的分叉节点及其索引（如果存在）
//否则零
//打电话的人应该锁着
func (t *Pot) getPos(po int) (n *Pot, i int) {
	for i, n = range t.bins {
		if po > n.po {
			continue
		}
		if po < n.po {
			return nil, i
		}
		return n, i
	}
	return nil, len(t.bins)
}

//需要调用（m，max，extra）使用max m out extra，然后使用max
//如果需要，返回调整后的计数
func need(m, max, extra int) (int, int, int) {
	if m <= extra {
		return m, max, extra - m
	}
	max += extra - m
	if max <= 0 {
		return m + max, 0, 0
	}
	return m, max, 0
}

func (t *Pot) String() string {
	return t.sstring("")
}

func (t *Pot) sstring(indent string) string {
	if t == nil {
		return "<nil>"
	}
	var s string
	indent += "  "
	s += fmt.Sprintf("%v%v (%v) %v \n", indent, t.pin, t.po, t.size)
	for _, n := range t.bins {
		s += fmt.Sprintf("%v%v\n", indent, n.sstring(indent))
	}
	return s
}
