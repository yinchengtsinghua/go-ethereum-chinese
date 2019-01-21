
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
package storage

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/metrics"
	ch "github.com/ethereum/go-ethereum/swarm/chunk"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/spancontext"
	opentracing "github.com/opentracing/opentracing-go"
	olog "github.com/opentracing/opentracing-go/log"
)

/*
此包中实现的分布式存储需要固定大小的内容块。

chunker是一个组件的接口，该组件负责分解和组装较大的数据。

TreeChunker基于树结构实现一个chunker，定义如下：

1树中的每个节点（包括根节点和其他分支节点）都存储为一个块。

2个分支节点对数据内容进行编码，这些内容包括节点下的整个子树所覆盖的数据切片大小及其所有子节点的哈希键：
数据i：=大小（子树i）键j键j+1…..Ki{{J+N-1}

3个叶节点对输入数据的实际子片进行编码。

4如果数据大小不超过最大chunk size，则数据存储在单个块中。
  键=哈希（Int64（大小）+数据）

5如果数据大小大于chunksize*分支^l，但不大于chunksize*
  分支^（L+1），数据向量被分割成块大小*
  分支长度（最后一个除外）。
  key=hash（int64（大小）+key（slice0）+key（slice1）+…）

 底层哈希函数是可配置的
**/


/*
树分块器是数据分块的具体实现。
这个chunker以一种简单的方式工作，它从文档中构建一个树，这样每个节点要么表示一块实际数据，要么表示一块表示树的分支非叶节点的数据。尤其是，每个这样的非叶块将表示其各自子块的散列的串联。该方案同时保证了数据的完整性和自寻址能力。抽象节点是透明的，因为其表示的大小组件严格大于其最大数据大小，因为它们编码子树。

如果一切正常，可以通过简单地编写读卡器来实现这一点，这样就不需要额外的分配或缓冲来进行数据拆分和连接。这意味着原则上，内存、文件系统、网络套接字之间可以有直接的IO（BZZ对等机存储请求是从套接字读取的）。实际上，可能需要几个阶段的内部缓冲。
不过，散列本身确实使用了额外的副本和分配，因为它确实需要它。
**/


type ChunkerParams struct {
	chunkSize int64
	hashSize  int64
}

type SplitterParams struct {
	ChunkerParams
	reader io.Reader
	putter Putter
	addr   Address
}

type TreeSplitterParams struct {
	SplitterParams
	size int64
}

type JoinerParams struct {
	ChunkerParams
	addr   Address
	getter Getter
//TODO:有一个bug，所以深度今天只能是0，请参见：https://github.com/ethersphere/go-ethereum/issues/344
	depth int
	ctx   context.Context
}

type TreeChunker struct {
	ctx context.Context

	branches int64
	dataSize int64
	data     io.Reader
//计算
	addr        Address
	depth       int
hashSize    int64        //self.hashfunc.new（）.size（）。
chunkSize   int64        //哈希大小*分支
workerCount int64        //使用的工作程序数
workerLock  sync.RWMutex //锁定工人计数
	jobC        chan *hashJob
	wg          *sync.WaitGroup
	putter      Putter
	getter      Getter
	errC        chan error
	quitC       chan bool
}

/*
 join基于根键重新构造原始内容。
 加入时，调用方将返回一个惰性的区段读取器，即
 可查找并实现按需获取块以及读取块的位置。
 要检索的新块来自调用方提供的getter。
 如果在联接过程中遇到错误，则显示为读卡器错误。
 区段阅读器。
 因此，即使其他部分也可以部分读取文档
 损坏或丢失。
 块在加入时不会被chunker验证。这个
 是因为由DPA决定哪些来源是可信的。
**/

func TreeJoin(ctx context.Context, addr Address, getter Getter, depth int) *LazyChunkReader {
	jp := &JoinerParams{
		ChunkerParams: ChunkerParams{
			chunkSize: ch.DefaultSize,
			hashSize:  int64(len(addr)),
		},
		addr:   addr,
		getter: getter,
		depth:  depth,
		ctx:    ctx,
	}

	return NewTreeJoiner(jp).Join(ctx)
}

/*
 拆分时，数据作为一个节阅读器提供，键是一个哈希大小的长字节片（键），一旦处理完成，整个内容的根哈希将填充此内容。
 要存储的新块是使用调用方提供的推杆存储的。
**/

func TreeSplit(ctx context.Context, data io.Reader, size int64, putter Putter) (k Address, wait func(context.Context) error, err error) {
	tsp := &TreeSplitterParams{
		SplitterParams: SplitterParams{
			ChunkerParams: ChunkerParams{
				chunkSize: ch.DefaultSize,
				hashSize:  putter.RefSize(),
			},
			reader: data,
			putter: putter,
		},
		size: size,
	}
	return NewTreeSplitter(tsp).Split(ctx)
}

func NewTreeJoiner(params *JoinerParams) *TreeChunker {
	tc := &TreeChunker{}
	tc.hashSize = params.hashSize
	tc.branches = params.chunkSize / params.hashSize
	tc.addr = params.addr
	tc.getter = params.getter
	tc.depth = params.depth
	tc.chunkSize = params.chunkSize
	tc.workerCount = 0
	tc.jobC = make(chan *hashJob, 2*ChunkProcessors)
	tc.wg = &sync.WaitGroup{}
	tc.errC = make(chan error)
	tc.quitC = make(chan bool)

	tc.ctx = params.ctx

	return tc
}

func NewTreeSplitter(params *TreeSplitterParams) *TreeChunker {
	tc := &TreeChunker{}
	tc.data = params.reader
	tc.dataSize = params.size
	tc.hashSize = params.hashSize
	tc.branches = params.chunkSize / params.hashSize
	tc.addr = params.addr
	tc.chunkSize = params.chunkSize
	tc.putter = params.putter
	tc.workerCount = 0
	tc.jobC = make(chan *hashJob, 2*ChunkProcessors)
	tc.wg = &sync.WaitGroup{}
	tc.errC = make(chan error)
	tc.quitC = make(chan bool)

	return tc
}

type hashJob struct {
	key      Address
	chunk    []byte
	size     int64
	parentWg *sync.WaitGroup
}

func (tc *TreeChunker) incrementWorkerCount() {
	tc.workerLock.Lock()
	defer tc.workerLock.Unlock()
	tc.workerCount += 1
}

func (tc *TreeChunker) getWorkerCount() int64 {
	tc.workerLock.RLock()
	defer tc.workerLock.RUnlock()
	return tc.workerCount
}

func (tc *TreeChunker) decrementWorkerCount() {
	tc.workerLock.Lock()
	defer tc.workerLock.Unlock()
	tc.workerCount -= 1
}

func (tc *TreeChunker) Split(ctx context.Context) (k Address, wait func(context.Context) error, err error) {
	if tc.chunkSize <= 0 {
		panic("chunker must be initialised")
	}

	tc.runWorker(ctx)

	depth := 0
	treeSize := tc.chunkSize

//取最小深度，使chunkSize*hashCount^（depth+1）>size
//幂级数，将找出数据大小在基散列计数中的数量级或结果树中分支级别的数量级。
	for ; treeSize < tc.dataSize; treeSize *= tc.branches {
		depth++
	}

	key := make([]byte, tc.hashSize)
//此waitgroup成员在计算根哈希之后释放
	tc.wg.Add(1)
//启动传递等待组的实际递归函数
	go tc.split(ctx, depth, treeSize/tc.branches, key, tc.dataSize, tc.wg)

//如果工作组中的所有子进程都已完成，则关闭内部错误通道
	go func() {
//等待所有线程完成
		tc.wg.Wait()
		close(tc.errC)
	}()

	defer close(tc.quitC)
	defer tc.putter.Close()
	select {
	case err := <-tc.errC:
		if err != nil {
			return nil, nil, err
		}
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}

	return key, tc.putter.Wait, nil
}

func (tc *TreeChunker) split(ctx context.Context, depth int, treeSize int64, addr Address, size int64, parentWg *sync.WaitGroup) {

//

	for depth > 0 && size < treeSize {
		treeSize /= tc.branches
		depth--
	}

	if depth == 0 {
//叶节点->内容块
		chunkData := make([]byte, size+8)
		binary.LittleEndian.PutUint64(chunkData[0:8], uint64(size))
		var readBytes int64
		for readBytes < size {
			n, err := tc.data.Read(chunkData[8+readBytes:])
			readBytes += int64(n)
			if err != nil && !(err == io.EOF && readBytes == size) {
				tc.errC <- err
				return
			}
		}
		select {
		case tc.jobC <- &hashJob{addr, chunkData, size, parentWg}:
		case <-tc.quitC:
		}
		return
	}
//部门> 0
//包含子节点哈希的中间块
	branchCnt := (size + treeSize - 1) / treeSize

	var chunk = make([]byte, branchCnt*tc.hashSize+8)
	var pos, i int64

	binary.LittleEndian.PutUint64(chunk[0:8], uint64(size))

	childrenWg := &sync.WaitGroup{}
	var secSize int64
	for i < branchCnt {
//最后一项可以有较短的数据
		if size-pos < treeSize {
			secSize = size - pos
		} else {
			secSize = treeSize
		}
//数据的散列值
		subTreeAddress := chunk[8+i*tc.hashSize : 8+(i+1)*tc.hashSize]

		childrenWg.Add(1)
		tc.split(ctx, depth-1, treeSize/tc.branches, subTreeAddress, secSize, childrenWg)

		i++
		pos += treeSize
	}
//等待所有子元素完成哈希计算并将其复制到块的各个部分
//添加（1）
//转到函数（）
	childrenWg.Wait()

	worker := tc.getWorkerCount()
	if int64(len(tc.jobC)) > worker && worker < ChunkProcessors {
		tc.runWorker(ctx)

	}
	select {
	case tc.jobC <- &hashJob{addr, chunk, size, parentWg}:
	case <-tc.quitC:
	}
}

func (tc *TreeChunker) runWorker(ctx context.Context) {
	tc.incrementWorkerCount()
	go func() {
		defer tc.decrementWorkerCount()
		for {
			select {

			case job, ok := <-tc.jobC:
				if !ok {
					return
				}

				h, err := tc.putter.Put(ctx, job.chunk)
				if err != nil {
					tc.errC <- err
					return
				}
				copy(job.key, h)
				job.parentWg.Done()
			case <-tc.quitC:
				return
			}
		}
	}()
}

//Lazychunkreader实现LazySectionReader
type LazyChunkReader struct {
	ctx       context.Context
addr      Address //根地址
	chunkData ChunkData
off       int64 //抵消
chunkSize int64 //从Chunker继承
branches  int64 //从Chunker继承
hashSize  int64 //从Chunker继承
	depth     int
	getter    Getter
}

func (tc *TreeChunker) Join(ctx context.Context) *LazyChunkReader {
	return &LazyChunkReader{
		addr:      tc.addr,
		chunkSize: tc.chunkSize,
		branches:  tc.branches,
		hashSize:  tc.hashSize,
		depth:     tc.depth,
		getter:    tc.getter,
		ctx:       tc.ctx,
	}
}

func (r *LazyChunkReader) Context() context.Context {
	return r.ctx
}

//大小将在LazySectionReader上调用
func (r *LazyChunkReader) Size(ctx context.Context, quitC chan bool) (n int64, err error) {
	metrics.GetOrRegisterCounter("lazychunkreader.size", nil).Inc(1)

	var sp opentracing.Span
	var cctx context.Context
	cctx, sp = spancontext.StartSpan(
		ctx,
		"lcr.size")
	defer sp.Finish()

	log.Debug("lazychunkreader.size", "addr", r.addr)
	if r.chunkData == nil {
		startTime := time.Now()
		chunkData, err := r.getter.Get(cctx, Reference(r.addr))
		if err != nil {
			metrics.GetOrRegisterResettingTimer("lcr.getter.get.err", nil).UpdateSince(startTime)
			return 0, err
		}
		metrics.GetOrRegisterResettingTimer("lcr.getter.get", nil).UpdateSince(startTime)
		r.chunkData = chunkData
	}

	s := r.chunkData.Size()
	log.Debug("lazychunkreader.size", "key", r.addr, "size", s)

	return int64(s), nil
}

//读在可以被称为无数次
//允许并发读取
//首先需要在Lazychunkreader上同步调用size（）。
func (r *LazyChunkReader) ReadAt(b []byte, off int64) (read int, err error) {
	metrics.GetOrRegisterCounter("lazychunkreader.readat", nil).Inc(1)

	var sp opentracing.Span
	var cctx context.Context
	cctx, sp = spancontext.StartSpan(
		r.ctx,
		"lcr.read")
	defer sp.Finish()

	defer func() {
		sp.LogFields(
			olog.Int("off", int(off)),
			olog.Int("read", read))
	}()

//这是正确的，swarm文档不能是零长度，因此不需要EOF
	if len(b) == 0 {
		return 0, nil
	}
	quitC := make(chan bool)
	size, err := r.Size(cctx, quitC)
	if err != nil {
		log.Debug("lazychunkreader.readat.size", "size", size, "err", err)
		return 0, err
	}

	errC := make(chan error)

//}
	var treeSize int64
	var depth int
//计算深度和最大树尺寸
	treeSize = r.chunkSize
	for ; treeSize < size; treeSize *= r.branches {
		depth++
	}
	wg := sync.WaitGroup{}
	length := int64(len(b))
	for d := 0; d < r.depth; d++ {
		off *= r.chunkSize
		length *= r.chunkSize
	}
	wg.Add(1)
	go r.join(b, off, off+length, depth, treeSize/r.branches, r.chunkData, &wg, errC, quitC)
	go func() {
		wg.Wait()
		close(errC)
	}()

	err = <-errC
	if err != nil {
		log.Debug("lazychunkreader.readat.errc", "err", err)
		close(quitC)
		return 0, err
	}
	if off+int64(len(b)) >= size {
		log.Debug("lazychunkreader.readat.return at end", "size", size, "off", off)
		return int(size - off), io.EOF
	}
	log.Debug("lazychunkreader.readat.errc", "buff", len(b))
	return len(b), nil
}

func (r *LazyChunkReader) join(b []byte, off int64, eoff int64, depth int, treeSize int64, chunkData ChunkData, parentWg *sync.WaitGroup, errC chan error, quitC chan bool) {
	defer parentWg.Done()
//查找适当的块级别
	for chunkData.Size() < uint64(treeSize) && depth > r.depth {
		treeSize /= r.branches
		depth--
	}

//找到叶块
	if depth == r.depth {
		extra := 8 + eoff - int64(len(chunkData))
		if extra > 0 {
			eoff -= extra
		}
		copy(b, chunkData[8+off:8+eoff])
return //只需将内容块返回给块阅读器
	}

//子树
	start := off / treeSize
	end := (eoff + treeSize - 1) / treeSize

//最后一个非叶块可以短于默认块大小，我们不要再进一步读取它的结尾
	currentBranches := int64(len(chunkData)-8) / r.hashSize
	if end > currentBranches {
		end = currentBranches
	}

	wg := &sync.WaitGroup{}
	defer wg.Wait()
	for i := start; i < end; i++ {
		soff := i * treeSize
		roff := soff
		seoff := soff + treeSize

		if soff < off {
			soff = off
		}
		if seoff > eoff {
			seoff = eoff
		}
		if depth > 1 {
			wg.Wait()
		}
		wg.Add(1)
		go func(j int64) {
			childAddress := chunkData[8+j*r.hashSize : 8+(j+1)*r.hashSize]
			startTime := time.Now()
			chunkData, err := r.getter.Get(r.ctx, Reference(childAddress))
			if err != nil {
				metrics.GetOrRegisterResettingTimer("lcr.getter.get.err", nil).UpdateSince(startTime)
				log.Debug("lazychunkreader.join", "key", fmt.Sprintf("%x", childAddress), "err", err)
				select {
				case errC <- fmt.Errorf("chunk %v-%v not found; key: %s", off, off+treeSize, fmt.Sprintf("%x", childAddress)):
				case <-quitC:
				}
				return
			}
			metrics.GetOrRegisterResettingTimer("lcr.getter.get", nil).UpdateSince(startTime)
			if l := len(chunkData); l < 9 {
				select {
				case errC <- fmt.Errorf("chunk %v-%v incomplete; key: %s, data length %v", off, off+treeSize, fmt.Sprintf("%x", childAddress), l):
				case <-quitC:
				}
				return
			}
			if soff < off {
				soff = off
			}
			r.join(b[soff-off:seoff-off], soff-roff, seoff-roff, depth-1, treeSize/r.branches, chunkData, wg, errC, quitC)
		}(i)
} //对于
}

//read保留一个光标，因此不能同时调用，请参阅readat
func (r *LazyChunkReader) Read(b []byte) (read int, err error) {
	log.Debug("lazychunkreader.read", "key", r.addr)
	metrics.GetOrRegisterCounter("lazychunkreader.read", nil).Inc(1)

	read, err = r.ReadAt(b, r.off)
	if err != nil && err != io.EOF {
		log.Debug("lazychunkreader.readat", "read", read, "err", err)
		metrics.GetOrRegisterCounter("lazychunkreader.read.err", nil).Inc(1)
	}

	metrics.GetOrRegisterCounter("lazychunkreader.read.bytes", nil).Inc(int64(read))

	r.off += int64(read)
	return read, err
}

//完全类似于标准的sectionreader实现
var errWhence = errors.New("Seek: invalid whence")
var errOffset = errors.New("Seek: invalid offset")

func (r *LazyChunkReader) Seek(offset int64, whence int) (int64, error) {
	log.Debug("lazychunkreader.seek", "key", r.addr, "offset", offset)
	switch whence {
	default:
		return 0, errWhence
	case 0:
		offset += 0
	case 1:
		offset += r.off
	case 2:
if r.chunkData == nil { //从结尾搜索要求rootchunk的大小。先调用大小
			_, err := r.Size(context.TODO(), nil)
			if err != nil {
				return 0, fmt.Errorf("can't get size: %v", err)
			}
		}
		offset += int64(r.chunkData.Size())
	}

	if offset < 0 {
		return 0, errOffset
	}
	r.off = offset
	return offset, nil
}
