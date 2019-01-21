
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
	"io"
	"io/ioutil"
	"sync"
	"time"

	ch "github.com/ethereum/go-ethereum/swarm/chunk"
	"github.com/ethereum/go-ethereum/swarm/log"
)

/*
   金字塔块的主要思想是在不知道整个大小的前提下处理输入数据。
   为了实现这一点，chunker树是从地面建立的，直到数据耗尽。
   这就打开了新的Aveneus，比如容易附加和对树进行其他类型的修改，从而避免了
   重复数据块。


   下面是一个两级块树的例子。叶块称为数据块，以上都称为
   块称为树块。数据块上方的树块为0级，以此类推，直到达到
   根目录树块。



                                            t10<-树块Lvl1
                                            γ
                  _uuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuu
                 /_
                /\ \
            _uuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuuu
           //\//\//\////\
          //\//\//\////\
         D1 D2…d128 d1 d2…d128 d1 d2…d128 d1 d2…D128<-数据块


    split函数连续读取数据并创建数据块并将其发送到存储器。
    当创建一定数量的数据块（默认分支）时，会发送一个信号来创建树。
    条目。当0级树条目达到某个阈值（默认分支）时，另一个信号
    发送到一级以上的树条目。等等…直到只有一个数据用尽
    树条目存在于某个级别。树条目的键作为文件的根地址给出。

**/


var (
	errLoadingTreeRootChunk = errors.New("LoadTree Error: Could not load root chunk")
	errLoadingTreeChunk     = errors.New("LoadTree Error: Could not load chunk")
)

const (
	ChunkProcessors = 8
	splitTimeout    = time.Minute * 5
)

type PyramidSplitterParams struct {
	SplitterParams
	getter Getter
}

func NewPyramidSplitterParams(addr Address, reader io.Reader, putter Putter, getter Getter, chunkSize int64) *PyramidSplitterParams {
	hashSize := putter.RefSize()
	return &PyramidSplitterParams{
		SplitterParams: SplitterParams{
			ChunkerParams: ChunkerParams{
				chunkSize: chunkSize,
				hashSize:  hashSize,
			},
			reader: reader,
			putter: putter,
			addr:   addr,
		},
		getter: getter,
	}
}

/*
 拆分时，数据作为一个节阅读器提供，键是一个hashsize长字节片（地址），一旦处理完成，整个内容的根散列将填充此内容。
 要存储的新块是使用调用方提供的推杆存储的。
**/

func PyramidSplit(ctx context.Context, reader io.Reader, putter Putter, getter Getter) (Address, func(context.Context) error, error) {
	return NewPyramidSplitter(NewPyramidSplitterParams(nil, reader, putter, getter, ch.DefaultSize)).Split(ctx)
}

func PyramidAppend(ctx context.Context, addr Address, reader io.Reader, putter Putter, getter Getter) (Address, func(context.Context) error, error) {
	return NewPyramidSplitter(NewPyramidSplitterParams(addr, reader, putter, getter, ch.DefaultSize)).Append(ctx)
}

//创建树节点的条目
type TreeEntry struct {
	level         int
	branchCount   int64
	subtreeSize   uint64
	chunk         []byte
	key           []byte
index         int  //在append中用于指示现有树条目的索引
updatePending bool //指示是否从现有树加载项
}

func NewTreeEntry(pyramid *PyramidChunker) *TreeEntry {
	return &TreeEntry{
		level:         0,
		branchCount:   0,
		subtreeSize:   0,
		chunk:         make([]byte, pyramid.chunkSize+8),
		key:           make([]byte, pyramid.hashSize),
		index:         0,
		updatePending: false,
	}
}

//哈希处理器用于创建数据/树块并发送到存储
type chunkJob struct {
	key      Address
	chunk    []byte
	parentWg *sync.WaitGroup
}

type PyramidChunker struct {
	chunkSize   int64
	hashSize    int64
	branches    int64
	reader      io.Reader
	putter      Putter
	getter      Getter
	key         Address
	workerCount int64
	workerLock  sync.RWMutex
	jobC        chan *chunkJob
	wg          *sync.WaitGroup
	errC        chan error
	quitC       chan bool
	rootAddress []byte
	chunkLevel  [][]*TreeEntry
}

func NewPyramidSplitter(params *PyramidSplitterParams) (pc *PyramidChunker) {
	pc = &PyramidChunker{}
	pc.reader = params.reader
	pc.hashSize = params.hashSize
	pc.branches = params.chunkSize / pc.hashSize
	pc.chunkSize = pc.hashSize * pc.branches
	pc.putter = params.putter
	pc.getter = params.getter
	pc.key = params.addr
	pc.workerCount = 0
	pc.jobC = make(chan *chunkJob, 2*ChunkProcessors)
	pc.wg = &sync.WaitGroup{}
	pc.errC = make(chan error)
	pc.quitC = make(chan bool)
	pc.rootAddress = make([]byte, pc.hashSize)
	pc.chunkLevel = make([][]*TreeEntry, pc.branches)
	return
}

func (pc *PyramidChunker) Join(addr Address, getter Getter, depth int) LazySectionReader {
	return &LazyChunkReader{
		addr:      addr,
		depth:     depth,
		chunkSize: pc.chunkSize,
		branches:  pc.branches,
		hashSize:  pc.hashSize,
		getter:    getter,
	}
}

func (pc *PyramidChunker) incrementWorkerCount() {
	pc.workerLock.Lock()
	defer pc.workerLock.Unlock()
	pc.workerCount += 1
}

func (pc *PyramidChunker) getWorkerCount() int64 {
	pc.workerLock.Lock()
	defer pc.workerLock.Unlock()
	return pc.workerCount
}

func (pc *PyramidChunker) decrementWorkerCount() {
	pc.workerLock.Lock()
	defer pc.workerLock.Unlock()
	pc.workerCount -= 1
}

func (pc *PyramidChunker) Split(ctx context.Context) (k Address, wait func(context.Context) error, err error) {
	log.Debug("pyramid.chunker: Split()")

	pc.wg.Add(1)
	pc.prepareChunks(ctx, false)

//如果工作组中的所有子进程都已完成，则关闭内部错误通道
	go func() {

//等待所有块完成
		pc.wg.Wait()

//我们在这里关闭errc，因为它被传递到下面的8个并行例程中。
//如果其中一个发生错误…那个特定的程序会引起错误…
//一旦它们都成功完成，控制权就回来了，我们可以在这里安全地关闭它。
		close(pc.errC)
	}()

	defer close(pc.quitC)
	defer pc.putter.Close()

	select {
	case err := <-pc.errC:
		if err != nil {
			return nil, nil, err
		}
	case <-ctx.Done():
_ = pc.putter.Wait(ctx) //？？？？
		return nil, nil, ctx.Err()
	}
	return pc.rootAddress, pc.putter.Wait, nil

}

func (pc *PyramidChunker) Append(ctx context.Context) (k Address, wait func(context.Context) error, err error) {
	log.Debug("pyramid.chunker: Append()")
//加载每个级别中最右侧的未完成树块
	pc.loadTree(ctx)

	pc.wg.Add(1)
	pc.prepareChunks(ctx, true)

//如果工作组中的所有子进程都已完成，则关闭内部错误通道
	go func() {

//等待所有块完成
		pc.wg.Wait()

		close(pc.errC)
	}()

	defer close(pc.quitC)
	defer pc.putter.Close()

	select {
	case err := <-pc.errC:
		if err != nil {
			return nil, nil, err
		}
	case <-time.NewTimer(splitTimeout).C:
	}

	return pc.rootAddress, pc.putter.Wait, nil

}

func (pc *PyramidChunker) processor(ctx context.Context, id int64) {
	defer pc.decrementWorkerCount()
	for {
		select {

		case job, ok := <-pc.jobC:
			if !ok {
				return
			}
			pc.processChunk(ctx, id, job)
		case <-pc.quitC:
			return
		}
	}
}

func (pc *PyramidChunker) processChunk(ctx context.Context, id int64, job *chunkJob) {
	log.Debug("pyramid.chunker: processChunk()", "id", id)

	ref, err := pc.putter.Put(ctx, job.chunk)
	if err != nil {
		select {
		case pc.errC <- err:
		case <-pc.quitC:
		}
	}

//向上一级报告此块的哈希（键对应于父块的正确子块）
	copy(job.key, ref)

//将新块发送到存储
	job.parentWg.Done()
}

func (pc *PyramidChunker) loadTree(ctx context.Context) error {
	log.Debug("pyramid.chunker: loadTree()")
//获取根块以获取总大小
	chunkData, err := pc.getter.Get(ctx, Reference(pc.key))
	if err != nil {
		return errLoadingTreeRootChunk
	}
	chunkSize := int64(chunkData.Size())
	log.Trace("pyramid.chunker: root chunk", "chunk.Size", chunkSize, "pc.chunkSize", pc.chunkSize)

//如果数据大小小于块…添加更新为挂起的父级
	if chunkSize <= pc.chunkSize {
		newEntry := &TreeEntry{
			level:         0,
			branchCount:   1,
			subtreeSize:   uint64(chunkSize),
			chunk:         make([]byte, pc.chunkSize+8),
			key:           make([]byte, pc.hashSize),
			index:         0,
			updatePending: true,
		}
		copy(newEntry.chunk[8:], pc.key)
		pc.chunkLevel[0] = append(pc.chunkLevel[0], newEntry)
		return nil
	}

	var treeSize int64
	var depth int
	treeSize = pc.chunkSize
	for ; treeSize < chunkSize; treeSize *= pc.branches {
		depth++
	}
	log.Trace("pyramid.chunker", "depth", depth)

//添加根块条目
	branchCount := int64(len(chunkData)-8) / pc.hashSize
	newEntry := &TreeEntry{
		level:         depth - 1,
		branchCount:   branchCount,
		subtreeSize:   uint64(chunkSize),
		chunk:         chunkData,
		key:           pc.key,
		index:         0,
		updatePending: true,
	}
	pc.chunkLevel[depth-1] = append(pc.chunkLevel[depth-1], newEntry)

//添加树的其余部分
	for lvl := depth - 1; lvl >= 1; lvl-- {

//todo（jmozah）：不是加载完成的分支，然后在末端修剪，
//首先避免装载它们
		for _, ent := range pc.chunkLevel[lvl] {
			branchCount = int64(len(ent.chunk)-8) / pc.hashSize
			for i := int64(0); i < branchCount; i++ {
				key := ent.chunk[8+(i*pc.hashSize) : 8+((i+1)*pc.hashSize)]
				newChunkData, err := pc.getter.Get(ctx, Reference(key))
				if err != nil {
					return errLoadingTreeChunk
				}
				newChunkSize := newChunkData.Size()
				bewBranchCount := int64(len(newChunkData)-8) / pc.hashSize
				newEntry := &TreeEntry{
					level:         lvl - 1,
					branchCount:   bewBranchCount,
					subtreeSize:   newChunkSize,
					chunk:         newChunkData,
					key:           key,
					index:         0,
					updatePending: true,
				}
				pc.chunkLevel[lvl-1] = append(pc.chunkLevel[lvl-1], newEntry)

			}

//我们只需要得到最右边未完成的分支。所以修剪所有完成的树枝
			if int64(len(pc.chunkLevel[lvl-1])) >= pc.branches {
				pc.chunkLevel[lvl-1] = nil
			}
		}
	}

	return nil
}

func (pc *PyramidChunker) prepareChunks(ctx context.Context, isAppend bool) {
	log.Debug("pyramid.chunker: prepareChunks", "isAppend", isAppend)
	defer pc.wg.Done()

	chunkWG := &sync.WaitGroup{}

	pc.incrementWorkerCount()

	go pc.processor(ctx, pc.workerCount)

	parent := NewTreeEntry(pc)
	var unfinishedChunkData ChunkData
	var unfinishedChunkSize uint64

	if isAppend && len(pc.chunkLevel[0]) != 0 {
		lastIndex := len(pc.chunkLevel[0]) - 1
		ent := pc.chunkLevel[0][lastIndex]

		if ent.branchCount < pc.branches {
			parent = &TreeEntry{
				level:         0,
				branchCount:   ent.branchCount,
				subtreeSize:   ent.subtreeSize,
				chunk:         ent.chunk,
				key:           ent.key,
				index:         lastIndex,
				updatePending: true,
			}

			lastBranch := parent.branchCount - 1
			lastAddress := parent.chunk[8+lastBranch*pc.hashSize : 8+(lastBranch+1)*pc.hashSize]

			var err error
			unfinishedChunkData, err = pc.getter.Get(ctx, lastAddress)
			if err != nil {
				pc.errC <- err
			}
			unfinishedChunkSize = unfinishedChunkData.Size()
			if unfinishedChunkSize < uint64(pc.chunkSize) {
				parent.subtreeSize = parent.subtreeSize - unfinishedChunkSize
				parent.branchCount = parent.branchCount - 1
			} else {
				unfinishedChunkData = nil
			}
		}
	}

	for index := 0; ; index++ {
		var err error
		chunkData := make([]byte, pc.chunkSize+8)

		var readBytes int

		if unfinishedChunkData != nil {
			copy(chunkData, unfinishedChunkData)
			readBytes += int(unfinishedChunkSize)
			unfinishedChunkData = nil
			log.Trace("pyramid.chunker: found unfinished chunk", "readBytes", readBytes)
		}

		var res []byte
		res, err = ioutil.ReadAll(io.LimitReader(pc.reader, int64(len(chunkData)-(8+readBytes))))

//ioutil.readall的黑客：
//对ioutil.readall的成功调用返回err==nil，not err==eof，而我们
//要传播IO.EOF错误
		if len(res) == 0 && err == nil {
			err = io.EOF
		}
		copy(chunkData[8+readBytes:], res)

		readBytes += len(res)
		log.Trace("pyramid.chunker: copied all data", "readBytes", readBytes)

		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {

				pc.cleanChunkLevels()

//检查是否追加，或者块是唯一的。
				if parent.branchCount == 1 && (pc.depth() == 0 || isAppend) {
//数据正好是一个块。选取最后一个区块键作为根
					chunkWG.Wait()
					lastChunksAddress := parent.chunk[8 : 8+pc.hashSize]
					copy(pc.rootAddress, lastChunksAddress)
					break
				}
			} else {
				close(pc.quitC)
				break
			}
		}

//数据以块边界结尾。只需发出信号，开始建造树木
		if readBytes == 0 {
			pc.buildTree(isAppend, parent, chunkWG, true, nil)
			break
		} else {
			pkey := pc.enqueueDataChunk(chunkData, uint64(readBytes), parent, chunkWG)

//更新与树相关的父数据结构
			parent.subtreeSize += uint64(readBytes)
			parent.branchCount++

//数据耗尽…发送任何与父树相关的块的信号
			if int64(readBytes) < pc.chunkSize {

				pc.cleanChunkLevels()

//只有一个数据块..所以不要添加任何父块
				if parent.branchCount <= 1 {
					chunkWG.Wait()

					if isAppend || pc.depth() == 0 {
//如果深度为0，则无需构建树
//或者我们正在追加。
//只用最后一把钥匙。
						copy(pc.rootAddress, pkey)
					} else {
//我们需要建造树和提供孤独
//chunk键替换最后一个树chunk键。
						pc.buildTree(isAppend, parent, chunkWG, true, pkey)
					}
					break
				}

				pc.buildTree(isAppend, parent, chunkWG, true, nil)
				break
			}

			if parent.branchCount == pc.branches {
				pc.buildTree(isAppend, parent, chunkWG, false, nil)
				parent = NewTreeEntry(pc)
			}

		}

		workers := pc.getWorkerCount()
		if int64(len(pc.jobC)) > workers && workers < ChunkProcessors {
			pc.incrementWorkerCount()
			go pc.processor(ctx, pc.workerCount)
		}

	}

}

func (pc *PyramidChunker) buildTree(isAppend bool, ent *TreeEntry, chunkWG *sync.WaitGroup, last bool, lonelyChunkKey []byte) {
	chunkWG.Wait()
	pc.enqueueTreeChunk(ent, chunkWG, last)

	compress := false
	endLvl := pc.branches
	for lvl := int64(0); lvl < pc.branches; lvl++ {
		lvlCount := int64(len(pc.chunkLevel[lvl]))
		if lvlCount >= pc.branches {
			endLvl = lvl + 1
			compress = true
			break
		}
	}

	if !compress && !last {
		return
	}

//在压缩树之前，请等待所有要处理的键
	chunkWG.Wait()

	for lvl := int64(ent.level); lvl < endLvl; lvl++ {

		lvlCount := int64(len(pc.chunkLevel[lvl]))
		if lvlCount == 1 && last {
			copy(pc.rootAddress, pc.chunkLevel[lvl][0].key)
			return
		}

		for startCount := int64(0); startCount < lvlCount; startCount += pc.branches {

			endCount := startCount + pc.branches
			if endCount > lvlCount {
				endCount = lvlCount
			}

			var nextLvlCount int64
			var tempEntry *TreeEntry
			if len(pc.chunkLevel[lvl+1]) > 0 {
				nextLvlCount = int64(len(pc.chunkLevel[lvl+1]) - 1)
				tempEntry = pc.chunkLevel[lvl+1][nextLvlCount]
			}
			if isAppend && tempEntry != nil && tempEntry.updatePending {
				updateEntry := &TreeEntry{
					level:         int(lvl + 1),
					branchCount:   0,
					subtreeSize:   0,
					chunk:         make([]byte, pc.chunkSize+8),
					key:           make([]byte, pc.hashSize),
					index:         int(nextLvlCount),
					updatePending: true,
				}
				for index := int64(0); index < lvlCount; index++ {
					updateEntry.branchCount++
					updateEntry.subtreeSize += pc.chunkLevel[lvl][index].subtreeSize
					copy(updateEntry.chunk[8+(index*pc.hashSize):8+((index+1)*pc.hashSize)], pc.chunkLevel[lvl][index].key[:pc.hashSize])
				}

				pc.enqueueTreeChunk(updateEntry, chunkWG, last)

			} else {

				noOfBranches := endCount - startCount
				newEntry := &TreeEntry{
					level:         int(lvl + 1),
					branchCount:   noOfBranches,
					subtreeSize:   0,
					chunk:         make([]byte, (noOfBranches*pc.hashSize)+8),
					key:           make([]byte, pc.hashSize),
					index:         int(nextLvlCount),
					updatePending: false,
				}

				index := int64(0)
				for i := startCount; i < endCount; i++ {
					entry := pc.chunkLevel[lvl][i]
					newEntry.subtreeSize += entry.subtreeSize
					copy(newEntry.chunk[8+(index*pc.hashSize):8+((index+1)*pc.hashSize)], entry.key[:pc.hashSize])
					index++
				}
//孤独区块键是最后一个区块的键，在最后一个分支上只有一个区块。
//在这种情况下，忽略ITS树块键并将其替换为孤独块键。
				if lonelyChunkKey != nil {
//用Lonely数据块键覆盖最后一个树块键。
					copy(newEntry.chunk[int64(len(newEntry.chunk))-pc.hashSize:], lonelyChunkKey[:pc.hashSize])
				}

				pc.enqueueTreeChunk(newEntry, chunkWG, last)

			}

		}

		if !isAppend {
			chunkWG.Wait()
			if compress {
				pc.chunkLevel[lvl] = nil
			}
		}
	}

}

func (pc *PyramidChunker) enqueueTreeChunk(ent *TreeEntry, chunkWG *sync.WaitGroup, last bool) {
	if ent != nil && ent.branchCount > 0 {

//在处理树块之前，请等待数据块通过。
		if last {
			chunkWG.Wait()
		}

		binary.LittleEndian.PutUint64(ent.chunk[:8], ent.subtreeSize)
		ent.key = make([]byte, pc.hashSize)
		chunkWG.Add(1)
		select {
		case pc.jobC <- &chunkJob{ent.key, ent.chunk[:ent.branchCount*pc.hashSize+8], chunkWG}:
		case <-pc.quitC:
		}

//根据天气情况更新或附加它是一个新条目或被重用
		if ent.updatePending {
			chunkWG.Wait()
			pc.chunkLevel[ent.level][ent.index] = ent
		} else {
			pc.chunkLevel[ent.level] = append(pc.chunkLevel[ent.level], ent)
		}

	}
}

func (pc *PyramidChunker) enqueueDataChunk(chunkData []byte, size uint64, parent *TreeEntry, chunkWG *sync.WaitGroup) Address {
	binary.LittleEndian.PutUint64(chunkData[:8], size)
	pkey := parent.chunk[8+parent.branchCount*pc.hashSize : 8+(parent.branchCount+1)*pc.hashSize]

	chunkWG.Add(1)
	select {
	case pc.jobC <- &chunkJob{pkey, chunkData[:size+8], chunkWG}:
	case <-pc.quitC:
	}

	return pkey

}

//深度返回块级别的数目。
//它用于检测是否只有一个数据块
//最后一个分支。
func (pc *PyramidChunker) depth() (d int) {
	for _, l := range pc.chunkLevel {
		if l == nil {
			return
		}
		d++
	}
	return
}

//cleanchunklevels删除块级别之间的间隙（零级别）
//这不是零。
func (pc *PyramidChunker) cleanChunkLevels() {
	for i, l := range pc.chunkLevel {
		if l == nil {
			pc.chunkLevel = append(pc.chunkLevel[:i], append(pc.chunkLevel[i+1:], nil)...)
		}
	}
}
