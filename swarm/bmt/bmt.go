
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

//包bmt提供了一个用于swarm块散列的二进制merkle树实现
package bmt

import (
	"fmt"
	"hash"
	"strings"
	"sync"
	"sync/atomic"
)

/*
binary merkle tree hash是一个针对有限大小的任意数据块的哈希函数。
它被定义为在固定大小的段上构建的二进制merkle树的根散列
使用任何基哈希函数（例如，keccak 256 sha3）的底层块。
数据长度小于固定大小的块被散列，就像它们没有填充一样。

BMT散列被用作群中的块散列函数，而块散列函数又是
128分支群哈希http://swarm guide.readthedocs.io/en/latest/architecture.html swarm哈希

BMT最适合提供紧凑的包含证明，即证明
段是从特定偏移量开始的块的子字符串。
基础段的大小固定为基哈希（称为分辨率）的大小
在bmt散列中），使用keccak256 sha3散列为32个字节，evm字大小为链上bmt验证优化。
以及群散列的Merkle树中包含证明的最佳散列大小。

提供了两种实现：

*refhasher针对代码简单性进行了优化，是一种参考实现。
  这很容易理解
*哈希优化了速度，利用了最低限度的并发性
  协调并发例程的控制结构

  bmt hasher实现以下接口
 *标准golang hash.hash-同步，可重复使用
 *swarmhash-提供跨度
 *IO.WRITER-从左到右同步数据编写器
 *AsyncWriter-并发节写入和异步SUM调用
**/


const (
//池大小是散列程序使用的最大BMT树数，即，
//由同一哈希程序执行的最大并发BMT哈希操作数
	PoolSize = 8
)

//basehasherfunc是用于bmt的基哈希的hash.hash构造函数函数。
//由keccak256 sha3.newlegacykeccak256实施
type BaseHasherFunc func() hash.Hash

//hasher用于表示BMT的固定最大大小块的可重用hasher
//-实现hash.hash接口
//-重新使用一个树池进行缓冲内存分配和资源控制
//-支持顺序不可知的并发段写入和段（双段）写入
//以及顺序读写
//-不能对多个块同时调用同一哈希实例
//-同一哈希实例可同步重用
//-SUM将树还给游泳池并保证离开
//树及其自身处于可重用的状态，用于散列新块
//-生成和验证段包含证明（TODO:）
type Hasher struct {
pool *TreePool //BMT资源池
bmt  *tree     //用于流程控制和验证的预建BMT资源
}

//new创建一个可重用的bmt散列器，
//从资源池中提取新的树，以便对每个块进行哈希处理
func New(p *TreePool) *Hasher {
	return &Hasher{
		pool: p,
	}
}

//Treepool提供了BMT哈希程序用作资源的一个树池。
//从池中弹出的一棵树保证处于干净状态。
//用于散列新块。
type TreePool struct {
	lock         sync.Mutex
c            chan *tree     //从池中获取资源的通道
hasher       BaseHasherFunc //用于BMT级别的基本哈希器
SegmentSize  int            //叶段的大小，规定为=哈希大小
SegmentCount int            //BMT基准面上的段数
Capacity     int            //池容量，控制并发性
Depth        int            //bmt树的深度=int（log2（segmentcount））+1
Size         int            //数据的总长度（计数*大小）
count        int            //当前（曾经）分配的资源计数
zerohashes   [][]byte       //所有级别的可预测填充子树的查找表
}

//newtreepool创建一个树池，其中包含哈希、段大小、段计数和容量
//在hasher.gettree上，它重用空闲的树，或者在未达到容量时创建一个新的树。
func NewTreePool(hasher BaseHasherFunc, segmentCount, capacity int) *TreePool {
//初始化零哈希查找表
	depth := calculateDepthFor(segmentCount)
	segmentSize := hasher().Size()
	zerohashes := make([][]byte, depth+1)
	zeros := make([]byte, segmentSize)
	zerohashes[0] = zeros
	h := hasher()
	for i := 1; i < depth+1; i++ {
		zeros = doSum(h, nil, zeros, zeros)
		zerohashes[i] = zeros
	}
	return &TreePool{
		c:            make(chan *tree, capacity),
		hasher:       hasher,
		SegmentSize:  segmentSize,
		SegmentCount: segmentCount,
		Capacity:     capacity,
		Size:         segmentCount * segmentSize,
		Depth:        depth,
		zerohashes:   zerohashes,
	}
}

//排干水池中的水，直到其资源不超过n个。
func (p *TreePool) Drain(n int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	for len(p.c) > n {
		<-p.c
		p.count--
	}
}

//保留正在阻止，直到它返回可用的树
//它重用空闲的树，或者在未达到大小时创建一个新的树。
//TODO:应在此处使用上下文
func (p *TreePool) reserve() *tree {
	p.lock.Lock()
	defer p.lock.Unlock()
	var t *tree
	if p.count == p.Capacity {
		return <-p.c
	}
	select {
	case t = <-p.c:
	default:
		t = newTree(p.SegmentSize, p.Depth, p.hasher)
		p.count++
	}
	return t
}

//释放会将树放回池中。
//此树保证处于可重用状态
func (p *TreePool) release(t *tree) {
p.c <- t //永远不会失败…
}

//树是表示BMT的可重用控制结构
//以二叉树组织
//散列器使用treepool为每个块散列获取树
//树不在池中时被“锁定”
type tree struct {
leaves  []*node     //树的叶节点，通过父链接可访问的其他节点
cursor  int         //当前最右侧打开段的索引
offset  int         //当前打开段内的偏移量（光标位置）
section []byte      //最右边的开口段（双段）
result  chan []byte //结果通道
span    []byte      //包含在块下的数据范围
}

//节点是表示BMT中节点的可重用段散列器。
type node struct {
isLeft      bool      //是否为父双段的左侧
parent      *node     //指向BMT中父节点的指针
state       int32     //原子增量impl并发布尔切换
left, right []byte    //这是写两个孩子部分的地方
hasher      hash.Hash //节点上的预构造哈希
}

//newnode在bmt中构造一个段散列器节点（由newtree使用）
func newNode(index int, parent *node, hasher hash.Hash) *node {
	return &node{
		parent: parent,
		isLeft: index%2 == 0,
		hasher: hasher,
	}
}

//抽签抽BMT（不好）
func (t *tree) draw(hash []byte) string {
	var left, right []string
	var anc []*node
	for i, n := range t.leaves {
		left = append(left, fmt.Sprintf("%v", hashstr(n.left)))
		if i%2 == 0 {
			anc = append(anc, n.parent)
		}
		right = append(right, fmt.Sprintf("%v", hashstr(n.right)))
	}
	anc = t.leaves
	var hashes [][]string
	for l := 0; len(anc) > 0; l++ {
		var nodes []*node
		hash := []string{""}
		for i, n := range anc {
			hash = append(hash, fmt.Sprintf("%v|%v", hashstr(n.left), hashstr(n.right)))
			if i%2 == 0 && n.parent != nil {
				nodes = append(nodes, n.parent)
			}
		}
		hash = append(hash, "")
		hashes = append(hashes, hash)
		anc = nodes
	}
	hashes = append(hashes, []string{"", fmt.Sprintf("%v", hashstr(hash)), ""})
	total := 60
	del := "                             "
	var rows []string
	for i := len(hashes) - 1; i >= 0; i-- {
		var textlen int
		hash := hashes[i]
		for _, s := range hash {
			textlen += len(s)
		}
		if total < textlen {
			total = textlen + len(hash)
		}
		delsize := (total - textlen) / (len(hash) - 1)
		if delsize > len(del) {
			delsize = len(del)
		}
		row := fmt.Sprintf("%v: %v", len(hashes)-i-1, strings.Join(hash, del[:delsize]))
		rows = append(rows, row)

	}
	rows = append(rows, strings.Join(left, "  "))
	rows = append(rows, strings.Join(right, "  "))
	return strings.Join(rows, "\n") + "\n"
}

//NewTree通过构建BMT的节点初始化树
//-段大小规定为哈希的大小
func newTree(segmentSize, depth int, hashfunc func() hash.Hash) *tree {
	n := newNode(0, nil, hashfunc())
	prevlevel := []*node{n}
//迭代级别并创建2^（深度级别）节点
//0级位于双段段，因此我们从深度-2开始，因为
	count := 2
	for level := depth - 2; level >= 0; level-- {
		nodes := make([]*node, count)
		for i := 0; i < count; i++ {
			parent := prevlevel[i/2]
			var hasher hash.Hash
			if level == 0 {
				hasher = hashfunc()
			}
			nodes[i] = newNode(i, parent, hasher)
		}
		prevlevel = nodes
		count *= 2
	}
//datanode级别是最后一级的节点
	return &tree{
		leaves:  prevlevel,
		result:  make(chan []byte),
		section: make([]byte, 2*segmentSize),
	}
}

//实现hash.hash所需的方法

//SIZE返回大小
func (h *Hasher) Size() int {
	return h.pool.SegmentSize
}

//块大小返回块大小
func (h *Hasher) BlockSize() int {
	return 2 * h.pool.SegmentSize
}

//sum返回缓冲区的bmt根哈希
//使用SUM预先假定顺序同步写入（io.writer接口）
//hash.hash接口sum方法将字节片附加到基础
//在计算和返回块散列之前的数据
//调用方必须确保SUM不与WRITE、WRITESECTION同时调用
func (h *Hasher) Sum(b []byte) (s []byte) {
	t := h.getTree()
//将最后一节的final标志设置为true
	go h.writeSection(t.cursor, t.section, true, true)
//等待结果
	s = <-t.result
	span := t.span
//将树资源释放回池
	h.releaseTree()
//B+Sha3（SPAN+BMT（纯块）
	if len(span) == 0 {
		return append(b, s...)
	}
	return doSum(h.pool.hasher(), b, span, s)
}

//实现swarmhash和io.writer接口所需的方法

//按顺序写入调用，添加到要散列的缓冲区，
//在Go例程中的每个完整段调用中都有writesection
func (h *Hasher) Write(b []byte) (int, error) {
	l := len(b)
	if l == 0 || l > h.pool.Size {
		return 0, nil
	}
	t := h.getTree()
	secsize := 2 * h.pool.SegmentSize
//计算缺失位的长度以完成当前开放段
	smax := secsize - t.offset
//如果在块的开头或节的中间
	if t.offset < secsize {
//从缓冲区填充当前段
		copy(t.section[t.offset:], b)
//如果输入缓冲区被占用，并且打开的部分不完整，则
//提前抵销和返还
		if smax == 0 {
			smax = secsize
		}
		if l <= smax {
			t.offset += l
			return l, nil
		}
	} else {
//如果节的结尾
		if t.cursor == h.pool.SegmentCount*2 {
			return 0, nil
		}
	}
//从输入缓冲区读取完整部分和最后一个可能的部分部分。
	for smax < l {
//部分完成；异步推到树
		go h.writeSection(t.cursor, t.section, true, false)
//复位段
		t.section = make([]byte, secsize)
//从smax的输入缓冲区复制到节的右半部分
		copy(t.section, b[smax:])
//前进光标
		t.cursor++
//此处的smax表示输入缓冲区中的连续偏移量
		smax += secsize
	}
	t.offset = l - smax + secsize
	return l, nil
}

//在写入哈希之前需要调用Reset。
func (h *Hasher) Reset() {
	h.releaseTree()
}

//实现swarmhash接口所需的方法

//在写入哈希之前需要调用ResetWithLength
//参数应该是的字节片二进制表示。
//哈希下包含的数据长度，即跨度
func (h *Hasher) ResetWithLength(span []byte) {
	h.Reset()
	h.getTree().span = span
}

//releasetree将树放回池中解锁
//它重置树、段和索引
func (h *Hasher) releaseTree() {
	t := h.bmt
	if t == nil {
		return
	}
	h.bmt = nil
	go func() {
		t.cursor = 0
		t.offset = 0
		t.span = nil
		t.section = make([]byte, h.pool.SegmentSize*2)
		select {
		case <-t.result:
		default:
		}
		h.pool.release(t)
	}()
}

//NewAsyncWriter使用一个接口扩展哈希，用于并发段/节写入
func (h *Hasher) NewAsyncWriter(double bool) *AsyncHasher {
	secsize := h.pool.SegmentSize
	if double {
		secsize *= 2
	}
	write := func(i int, section []byte, final bool) {
		h.writeSection(i, section, double, final)
	}
	return &AsyncHasher{
		Hasher:  h,
		double:  double,
		secsize: secsize,
		write:   write,
	}
}

//节编写器是异步段/节编写器接口
type SectionWriter interface {
Reset()                                       //在重用前调用标准init
Write(index int, data []byte)                 //写入索引部分
Sum(b []byte, length int, span []byte) []byte //返回缓冲区的哈希值
SectionSize() int                             //要使用的异步节单元的大小
}

//AsyncHasher使用异步段/节编写器接口扩展BMT哈希器
//AsyncHasher不安全，不检查索引和节数据长度
//它必须与正确的索引和长度以及正确的节数一起使用
//
//如果
//*非最终截面比secsize短或长
//*如果最后一部分与长度不匹配
//*编写索引大于长度/秒大小的节
//*当length/secsize<maxsec时，在sum调用中设置长度
//
//*如果未对完全写入的哈希表调用sum（）。
//一个进程将阻塞，可通过重置终止。
//*如果不是所有部分都写了，但它会阻塞，则不会泄漏进程。
//并保留可释放的资源，调用reset（）。
type AsyncHasher struct {
*Hasher            //扩展哈希
mtx     sync.Mutex //锁定光标访问
double  bool       //是否使用双段（调用hasher.writesection）
secsize int        //基节大小（哈希或双精度大小）
	write   func(i int, section []byte, final bool)
}

//实现AsyncWriter所需的方法

//SECTIONSIZE返回要使用的异步节单元的大小
func (sw *AsyncHasher) SectionSize() int {
	return sw.secsize
}

//写入BMT基的第i部分
//此函数可以并打算同时调用
//它安全地设置最大段
func (sw *AsyncHasher) Write(i int, section []byte) {
	sw.mtx.Lock()
	defer sw.mtx.Unlock()
	t := sw.getTree()
//光标跟踪迄今为止写入的最右边的部分
//如果索引低于光标，则只需按原样编写非最终部分。
	if i < t.cursor {
//如果索引不是最右边的，则可以安全地写入节
		go sw.write(i, section, false)
		return
	}
//如果有上一个最右边的节可安全写入
	if t.offset > 0 {
		if i == t.cursor {
//i==cursor表示游标是通过哈希调用设置的，因此我们可以将节写入最后一个节。
//因为它可能更短，所以我们首先将它复制到填充缓冲区
			t.section = make([]byte, sw.secsize)
			copy(t.section, section)
			go sw.write(i, t.section, true)
			return
		}
//最右边的部分刚刚改变，所以我们把前一部分写为非最终部分。
		go sw.write(t.cursor, t.section, false)
	}
//将i设置为迄今为止编写的最基本部分的索引
//将t.offset设置为cursor*secsize+1
	t.cursor = i
	t.offset = i*sw.secsize + 1
	t.section = make([]byte, sw.secsize)
	copy(t.section, section)
}

//一旦长度和跨度已知，可以随时调用sum。
//甚至在所有段都被写入之前
//在这种情况下，SUM将阻塞，直到所有段都存在，并且
//可以计算长度的哈希值。
//
//B：摘要附加到B
//长度：输入的已知长度（不安全；超出范围时未定义）
//meta:要与最终摘要的bmt根一起哈希的元数据
//例如，防止存在伪造的跨度
func (sw *AsyncHasher) Sum(b []byte, length int, meta []byte) (s []byte) {
	sw.mtx.Lock()
	t := sw.getTree()
	if length == 0 {
		sw.mtx.Unlock()
		s = sw.pool.zerohashes[sw.pool.Depth]
	} else {
//对于非零输入，最右边的部分异步写入树
//如果写入了实际的最后一节（t.cursor==length/t.secsize）
		maxsec := (length - 1) / sw.secsize
		if t.offset > 0 {
			go sw.write(t.cursor, t.section, maxsec == t.cursor)
		}
//将光标设置为maxsec，以便在最后一节到达时写入它
		t.cursor = maxsec
		t.offset = length
		result := t.result
		sw.mtx.Unlock()
//等待结果或重置
		s = <-result
	}
//把树放回水池里
	sw.releaseTree()
//如果没有给定元，只需将摘要附加到b
	if len(meta) == 0 {
		return append(b, s...)
	}
//使用池一起散列meta和bmt根散列
	return doSum(sw.pool.hasher(), b, meta, s)
}

//WriteSection将第i个节的哈希写入BMT树的1级节点
func (h *Hasher) writeSection(i int, section []byte, double bool, final bool) {
//选择节的叶节点
	var n *node
	var isLeft bool
	var hasher hash.Hash
	var level int
	t := h.getTree()
	if double {
		level++
		n = t.leaves[i]
		hasher = n.hasher
		isLeft = n.isLeft
		n = n.parent
//散列该节
		section = doSum(hasher, nil, section)
	} else {
		n = t.leaves[i/2]
		hasher = n.hasher
		isLeft = i%2 == 0
	}
//将哈希写入父节点
	if final {
//对于最后一段，使用writefinalnode
		h.writeFinalNode(level, n, hasher, isLeft, section)
	} else {
		h.writeNode(n, hasher, isLeft, section)
	}
}

//WriteNode将数据推送到节点
//如果是两个姐妹中的第一个写的，程序就终止了
//如果是第二个，则计算散列并写入散列
//递归到父节点
//由于对父对象进行哈希操作是同步的，因此可以使用相同的哈希操作。
func (h *Hasher) writeNode(n *node, bh hash.Hash, isLeft bool, s []byte) {
	level := 1
	for {
//在bmt的根目录下，只需将结果写入结果通道
		if n == nil {
			h.getTree().result <- s
			return
		}
//否则，将子哈希赋给左或右段
		if isLeft {
			n.left = s
		} else {
			n.right = s
		}
//首先到达的子线程将终止
		if n.toggle() {
			return
		}
//现在第二个线程可以确保左孩子和右孩子都被写入
//所以它计算左右的散列值并将其推送到父对象
		s = doSum(bh, nil, n.left, n.right)
		isLeft = n.isLeft
		n = n.parent
		level++
	}
}

//WriteFinalNode正在沿着从最终数据集到
//通过父级的bmt根
//对于不平衡树，它使用
//所有零部分的bmt子树根散列的池查找表
//否则行为类似于“writenode”
func (h *Hasher) writeFinalNode(level int, n *node, bh hash.Hash, isLeft bool, s []byte) {

	for {
//在bmt的根目录下，只需将结果写入结果通道
		if n == nil {
			if s != nil {
				h.getTree().result <- s
			}
			return
		}
		var noHash bool
		if isLeft {
//来自左姊妹枝
//当最后一节的路径经过左子节点时
//我们为正确的级别包含一个全零子树散列并切换节点。
			n.right = h.pool.zerohashes[level]
			if s != nil {
				n.left = s
//如果左最后一个节点带有哈希，则它必须是第一个（并且只能是线程）
//所以开关已经处于被动状态，不需要呼叫
//然而线程需要继续向父线程推送哈希
				noHash = false
			} else {
//如果再次是第一个线程，那么传播nil并计算no hash
				noHash = n.toggle()
			}
		} else {
//右姐妹支
			if s != nil {
//如果从右子节点推送哈希，则写入右段更改状态
				n.right = s
//如果toggle为true，则我们首先到达，因此不需要散列，只需将nil推送到父级
				noHash = n.toggle()

			} else {
//如果s为nil，那么线程首先到达前一个节点，这里将有两个，
//所以不需要做任何事情，保持s=nil作为家长
				noHash = true
			}
		}
//第一个到达的子线程将继续重置为nil
//第二条线索现在可以确定左、右两个孩子都写了
//它计算左右的哈希并将其推送到父级
		if noHash {
			s = nil
		} else {
			s = doSum(bh, nil, n.left, n.right)
		}
//迭代到父级
		isLeft = n.isLeft
		n = n.parent
		level++
	}
}

//gettree通过从池中保留一个bmt资源并将其分配给bmt字段来获取bmt资源
func (h *Hasher) getTree() *tree {
	if h.bmt != nil {
		return h.bmt
	}
	t := h.pool.reserve()
	h.bmt = t
	return t
}

//实现并发可重用2状态对象的原子布尔切换
//具有%2的原子加载项实现原子布尔切换
//如果切换开关刚刚将其置于活动/等待状态，则返回true。
func (n *node) toggle() bool {
	return atomic.AddInt32(&n.state, 1)%2 == 1
}

//使用hash.hash计算数据的哈希
func doSum(h hash.Hash, b []byte, data ...[]byte) []byte {
	h.Reset()
	for _, v := range data {
		h.Write(v)
	}
	return h.Sum(b)
}

//hashstr是用于tree.draw中字节的漂亮打印机
func hashstr(b []byte) string {
	end := len(b)
	if end > 4 {
		end = 4
	}
	return fmt.Sprintf("%x", b[:end])
}

//CalculateDepthfor计算BMT树中的深度（层数）
func calculateDepthFor(n int) (d int) {
	c := 2
	for ; c < n; c *= 2 {
		d++
	}
	return d + 1
}
