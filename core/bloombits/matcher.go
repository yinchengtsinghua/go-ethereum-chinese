
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

package bloombits

import (
	"bytes"
	"context"
	"errors"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common/bitutil"
	"github.com/ethereum/go-ethereum/crypto"
)

//bloom indexes表示bloom过滤器中属于
//一些关键。
type bloomIndexes [3]uint

//calcBloomIndexes返回属于给定键的BloomFilter位索引。
func calcBloomIndexes(b []byte) bloomIndexes {
	b = crypto.Keccak256(b)

	var idxs bloomIndexes
	for i := 0; i < len(idxs); i++ {
		idxs[i] = (uint(b[2*i])<<8)&2047 + uint(b[2*i+1])
	}
	return idxs
}

//带有非零矢量的粒子图表示一个部分，其中一些子-
//匹配者已经找到了可能的匹配项。后续的子匹配者将
//二进制及其与该向量的匹配。如果矢量为零，则表示
//由第一个子匹配器处理的部分。
type partialMatches struct {
	section uint64
	bitset  []byte
}

//检索表示对给定任务的检索任务分配的请求
//具有给定数量的提取元素的位，或对这种请求的响应。
//它还可以将实际结果集用作传递数据结构。
//
//“竞赛”和“错误”字段由轻型客户端用于终止匹配
//如果在管道的某个路径上遇到错误，则为早期。
type Retrieval struct {
	Bit      uint
	Sections []uint64
	Bitsets  [][]byte

	Context context.Context
	Error   error
}

//Matcher是一个由调度程序和逻辑匹配程序组成的流水线系统，执行
//位流上的二进制和/或操作，创建一个潜在的流
//要检查数据内容的块。
type Matcher struct {
sectionSize uint64 //要筛选的数据批的大小

filters    [][]bloomIndexes    //系统匹配的筛选器
schedulers map[uint]*scheduler //用于装载钻头的检索调度程序

retrievers chan chan uint       //检索器处理等待位分配
counters   chan chan uint       //检索器进程正在等待任务计数报告
retrievals chan chan *Retrieval //检索器进程正在等待任务分配
deliveries chan *Retrieval      //检索器进程正在等待任务响应传递

running uint32 //原子标记会话是否处于活动状态
}

//NewMatcher创建了一个新的管道，用于检索Bloom位流并执行
//地址和主题过滤。将筛选器组件设置为“nil”是
//允许并将导致跳过该筛选规则（或0x11…1）。
func NewMatcher(sectionSize uint64, filters [][][]byte) *Matcher {
//创建Matcher实例
	m := &Matcher{
		sectionSize: sectionSize,
		schedulers:  make(map[uint]*scheduler),
		retrievers:  make(chan chan uint),
		counters:    make(chan chan uint),
		retrievals:  make(chan chan *Retrieval),
		deliveries:  make(chan *Retrieval),
	}
//计算我们感兴趣的组的bloom位索引
	m.filters = nil

	for _, filter := range filters {
//采集滤波规则的位指标，专用套管零滤波器
		if len(filter) == 0 {
			continue
		}
		bloomBits := make([]bloomIndexes, len(filter))
		for i, clause := range filter {
			if clause == nil {
				bloomBits = nil
				break
			}
			bloomBits[i] = calcBloomIndexes(clause)
		}
//如果不包含nil规则，则累积筛选规则
		if bloomBits != nil {
			m.filters = append(m.filters, bloomBits)
		}
	}
//对于每个位，创建一个调度程序来加载/下载位向量
	for _, bloomIndexLists := range m.filters {
		for _, bloomIndexList := range bloomIndexLists {
			for _, bloomIndex := range bloomIndexList {
				m.addScheduler(bloomIndex)
			}
		}
	}
	return m
}

//addscheduler为给定的位索引添加位流检索计划程序，如果
//它以前不存在。如果已选择位进行过滤，则
//可以使用现有的计划程序。
func (m *Matcher) addScheduler(idx uint) {
	if _, ok := m.schedulers[idx]; ok {
		return
	}
	m.schedulers[idx] = newScheduler(idx)
}

//Start启动匹配过程并返回一个bloom匹配流
//给定的块范围。如果范围内没有更多匹配项，则返回结果
//通道关闭。
func (m *Matcher) Start(ctx context.Context, begin, end uint64, results chan uint64) (*MatcherSession, error) {
//确保我们没有创建并发会话
	if atomic.SwapUint32(&m.running, 1) == 1 {
		return nil, errors.New("matcher already running")
	}
	defer atomic.StoreUint32(&m.running, 0)

//启动新的匹配轮
	session := &MatcherSession{
		matcher: m,
		quit:    make(chan struct{}),
		kill:    make(chan struct{}),
		ctx:     ctx,
	}
	for _, scheduler := range m.schedulers {
		scheduler.reset()
	}
	sink := m.run(begin, end, cap(results), session)

//从结果接收器读取输出并传递给用户
	session.pend.Add(1)
	go func() {
		defer session.pend.Done()
		defer close(results)

		for {
			select {
			case <-session.quit:
				return

			case res, ok := <-sink:
//找到新的匹配结果
				if !ok {
					return
				}
//计算该节的第一个和最后一个块
				sectionStart := res.section * m.sectionSize

				first := sectionStart
				if begin > first {
					first = begin
				}
				last := sectionStart + m.sectionSize - 1
				if end < last {
					last = end
				}
//遍历该节中的所有块并返回匹配的块
				for i := first; i <= last; i++ {
//如果在内部没有找到匹配项，则跳过整个字节（我们正在处理整个字节！）
					next := res.bitset[(i-sectionStart)/8]
					if next == 0 {
						if i%8 == 0 {
							i += 7
						}
						continue
					}
//设置一些位，做实际的子匹配
					if bit := 7 - i%8; next&(1<<bit) != 0 {
						select {
						case <-session.quit:
							return
						case results <- i:
						}
					}
				}
			}
		}
	}()
	return session, nil
}

//运行创建子匹配器的菊花链，一个用于地址集，另一个用于
//对于每个主题集，每个子匹配器仅在上一个主题集
//所有人都在该路段的一个街区找到了一个潜在的匹配点，
//然后二进制和ing自己匹配并将结果转发到下一个结果。
//
//该方法开始向上的第一个子匹配器提供节索引。
//并返回接收结果的接收通道。
func (m *Matcher) run(begin, end uint64, buffer int, session *MatcherSession) chan *partialMatches {
//创建源通道并将节索引馈送到
	source := make(chan *partialMatches, buffer)

	session.pend.Add(1)
	go func() {
		defer session.pend.Done()
		defer close(source)

		for i := begin / m.sectionSize; i <= end/m.sectionSize; i++ {
			select {
			case <-session.quit:
				return
			case source <- &partialMatches{i, bytes.Repeat([]byte{0xff}, int(m.sectionSize/8))}:
			}
		}
	}()
//组装菊花链过滤管道
	next := source
	dist := make(chan *request, buffer)

	for _, bloom := range m.filters {
		next = m.subMatch(next, dist, bloom, session)
	}
//启动请求分发
	session.pend.Add(1)
	go m.distributor(dist, session)

	return next
}

//Submatch创建一个子匹配器，该匹配器过滤一组地址或主题（二进制或-s），然后
//二进制和-s将结果发送到菊花链输入（源），并将其转发到菊花链输出。
//每个地址/主题的匹配是通过获取属于
//这个地址/主题，二进制和将这些向量组合在一起。
func (m *Matcher) subMatch(source chan *partialMatches, dist chan *request, bloom []bloomIndexes, session *MatcherSession) chan *partialMatches {
//为Bloom过滤器所需的每个位启动并发调度程序
	sectionSources := make([][3]chan uint64, len(bloom))
	sectionSinks := make([][3]chan []byte, len(bloom))
	for i, bits := range bloom {
		for j, bit := range bits {
			sectionSources[i][j] = make(chan uint64, cap(source))
			sectionSinks[i][j] = make(chan []byte, cap(source))

			m.schedulers[bit].run(sectionSources[i][j], dist, sectionSinks[i][j], session.quit, &session.pend)
		}
	}

process := make(chan *partialMatches, cap(source)) //在初始化提取之后，源中的条目将在此处转发。
	results := make(chan *partialMatches, cap(source))

	session.pend.Add(2)
	go func() {
//关闭Goroutine并终止所有源通道
		defer session.pend.Done()
		defer close(process)

		defer func() {
			for _, bloomSources := range sectionSources {
				for _, bitSource := range bloomSources {
					close(bitSource)
				}
			}
		}()
//从源信道中读取段，并多路复用到所有位调度程序中。
		for {
			select {
			case <-session.quit:
				return

			case subres, ok := <-source:
//从上一链接新建子结果
				if !ok {
					return
				}
//将区段索引多路复用到所有位调度程序
				for _, bloomSources := range sectionSources {
					for _, bitSource := range bloomSources {
						select {
						case <-session.quit:
							return
						case bitSource <- subres.section:
						}
					}
				}
//通知处理器此部分将可用
				select {
				case <-session.quit:
					return
				case process <- subres:
				}
			}
		}
	}()

	go func() {
//拆除Goroutine并终止最终水槽通道
		defer session.pend.Done()
		defer close(results)

//读取源通知并收集传递的结果
		for {
			select {
			case <-session.quit:
				return

			case subres, ok := <-process:
//已通知正在检索的节
				if !ok {
					return
				}
//收集所有子结果并将它们合并在一起
				var orVector []byte
				for _, bloomSinks := range sectionSinks {
					var andVector []byte
					for _, bitSink := range bloomSinks {
						var data []byte
						select {
						case <-session.quit:
							return
						case data = <-bitSink:
						}
						if andVector == nil {
							andVector = make([]byte, int(m.sectionSize/8))
							copy(andVector, data)
						} else {
							bitutil.ANDBytes(andVector, andVector, data)
						}
					}
					if orVector == nil {
						orVector = andVector
					} else {
						bitutil.ORBytes(orVector, orVector, andVector)
					}
				}

				if orVector == nil {
					orVector = make([]byte, int(m.sectionSize/8))
				}
				if subres.bitset != nil {
					bitutil.ANDBytes(orVector, orVector, subres.bitset)
				}
				if bitutil.TestBytes(orVector) {
					select {
					case <-session.quit:
						return
					case results <- &partialMatches{subres.section, orVector}:
					}
				}
			}
		}
	}()
	return results
}

//分发服务器接收来自调度程序的请求并将它们排队到一个集合中
//待处理的请求，这些请求被分配给想要完成它们的检索器。
func (m *Matcher) distributor(dist chan *request, session *MatcherSession) {
	defer session.pend.Done()

	var (
requests   = make(map[uint][]uint64) //按节号排序的节请求的逐位列表
unallocs   = make(map[uint]struct{}) //具有挂起请求但未分配给任何检索器的位
retrievers chan chan uint            //等待检索器（如果unallocs为空，则切换为nil）
	)
	var (
allocs   int            //处理正常关闭请求的活动分配数
shutdown = session.quit //关闭请求通道，将优雅地等待挂起的请求
	)

//assign是一个帮助方法，用于尝试将挂起的位
//监听服务，或者在有人到达后安排它。
	assign := func(bit uint) {
		select {
		case fetcher := <-m.retrievers:
			allocs++
			fetcher <- bit
		default:
//没有激活的检索器，开始监听新的检索器
			retrievers = m.retrievers
			unallocs[bit] = struct{}{}
		}
	}

	for {
		select {
		case <-shutdown:
//请求正常关闭，等待所有挂起的请求得到满足。
			if allocs == 0 {
				return
			}
			shutdown = nil

		case <-session.kill:
//未及时处理的未决请求，硬终止
			return

		case req := <-dist:
//到达新的检索请求，将其分发到某个提取进程
			queue := requests[req.bit]
			index := sort.Search(len(queue), func(i int) bool { return queue[i] >= req.section })
			requests[req.bit] = append(queue[:index], append([]uint64{req.section}, queue[index:]...)...)

//如果是一个新的位，我们有等待取数器，分配给他们
			if len(queue) == 0 {
				assign(req.bit)
			}

		case fetcher := <-retrievers:
//新的检索器到达，找到要分配的最下面的ed位。
			bit, best := uint(0), uint64(math.MaxUint64)
			for idx := range unallocs {
				if requests[idx][0] < best {
					bit, best = idx, requests[idx][0]
				}
			}
//停止跟踪此位（如果没有更多工作可用，则停止分配通知）
			delete(unallocs, bit)
			if len(unallocs) == 0 {
				retrievers = nil
			}
			allocs++
			fetcher <- bit

		case fetcher := <-m.counters:
//新任务计数请求已到达，返回项目数
			fetcher <- uint(len(requests[<-fetcher]))

		case fetcher := <-m.retrievals:
//等待任务检索、分配的新提取程序
			task := <-fetcher
			if want := len(task.Sections); want >= len(requests[task.Bit]) {
				task.Sections = requests[task.Bit]
				delete(requests, task.Bit)
			} else {
				task.Sections = append(task.Sections[:0], requests[task.Bit][:want]...)
				requests[task.Bit] = append(requests[task.Bit][:0], requests[task.Bit][want:]...)
			}
			fetcher <- task

//如果有未分配的内容，请尝试分配给其他人
			if len(requests[task.Bit]) > 0 {
				assign(task.Bit)
			}

		case result := <-m.deliveries:
//从获取器中新建检索任务响应，拆分缺少的部分和
//交付完整的
			var (
				sections = make([]uint64, 0, len(result.Sections))
				bitsets  = make([][]byte, 0, len(result.Bitsets))
				missing  = make([]uint64, 0, len(result.Sections))
			)
			for i, bitset := range result.Bitsets {
				if len(bitset) == 0 {
					missing = append(missing, result.Sections[i])
					continue
				}
				sections = append(sections, result.Sections[i])
				bitsets = append(bitsets, bitset)
			}
			m.schedulers[result.Bit].deliver(sections, bitsets)
			allocs--

//重新安排缺少的部分，如果新的部分可用，则分配位
			if len(missing) > 0 {
				queue := requests[result.Bit]
				for _, section := range missing {
					index := sort.Search(len(queue), func(i int) bool { return queue[i] >= section })
					queue = append(queue[:index], append([]uint64{section}, queue[index:]...)...)
				}
				requests[result.Bit] = queue

				if len(queue) == len(missing) {
					assign(result.Bit)
				}
			}
//如果我们正在关闭，请终止
			if allocs == 0 && shutdown == nil {
				return
			}
		}
	}
}

//MatcherSession由已启动的Matcher返回，用作终止符
//对于正在运行的匹配操作。
type MatcherSession struct {
	matcher *Matcher

closer sync.Once     //同步对象以确保只关闭一次
quit   chan struct{} //退出通道以请求管道终止
kill   chan struct{} //表示非正常强制停机的通道

ctx context.Context //轻型客户端用于中止筛选的上下文
err atomic.Value    //跟踪链深处检索失败的全局错误

	pend sync.WaitGroup
}

//关闭停止匹配进程并等待所有子进程终止
//返回前。超时可用于正常关机，允许
//当前正在运行要在此时间之前完成的检索。
func (s *MatcherSession) Close() {
	s.closer.Do(func() {
//信号终止并等待所有Goroutine关闭
		close(s.quit)
		time.AfterFunc(time.Second, func() { close(s.kill) })
		s.pend.Wait()
	})
}

//错误返回匹配会话期间遇到的任何失败。
func (s *MatcherSession) Error() error {
	if err := s.err.Load(); err != nil {
		return err.(error)
	}
	return nil
}

//allocateretrieval将一个bloom位索引分配给一个客户端进程，该进程可以
//立即请求并获取分配给该位的节内容或等待
//有一段时间需要更多的部分。
func (s *MatcherSession) AllocateRetrieval() (uint, bool) {
	fetcher := make(chan uint)

	select {
	case <-s.quit:
		return 0, false
	case s.matcher.retrievers <- fetcher:
		bit, ok := <-fetcher
		return bit, ok
	}
}

//PendingSections返回属于
//给定的牙轮钻头指数。
func (s *MatcherSession) PendingSections(bit uint) int {
	fetcher := make(chan uint)

	select {
	case <-s.quit:
		return 0
	case s.matcher.counters <- fetcher:
		fetcher <- bit
		return int(<-fetcher)
	}
}

//分配操作分配已分配的位任务队列的全部或部分
//到请求过程。
func (s *MatcherSession) AllocateSections(bit uint, count int) []uint64 {
	fetcher := make(chan *Retrieval)

	select {
	case <-s.quit:
		return nil
	case s.matcher.retrievals <- fetcher:
		task := &Retrieval{
			Bit:      bit,
			Sections: make([]uint64, count),
		}
		fetcher <- task
		return (<-fetcher).Sections
	}
}

//deliversections为特定的bloom提供一批区段位向量
//要注入处理管道的位索引。
func (s *MatcherSession) DeliverSections(bit uint, sections []uint64, bitsets [][]byte) {
	select {
	case <-s.kill:
		return
	case s.matcher.deliveries <- &Retrieval{Bit: bit, Sections: sections, Bitsets: bitsets}:
	}
}

//多路复用轮询匹配器会话以执行检索任务，并将其多路复用到
//请求的检索队列将与其他会话一起提供服务。
//
//此方法将在会话的生存期内阻塞。即使在终止之后
//在会议期间，任何在飞行中的请求都需要得到响应！空响应
//不过在那种情况下还是可以的。
func (s *MatcherSession) Multiplex(batch int, wait time.Duration, mux chan chan *Retrieval) {
	for {
//分配新的Bloom位索引以检索数据，完成后停止
		bit, ok := s.AllocateRetrieval()
		if !ok {
			return
		}
//位分配，如果低于批处理限制，则限制一点
		if s.PendingSections(bit) < batch {
			select {
			case <-s.quit:
//会话终止，我们无法有意义地服务，中止
				s.AllocateSections(bit, 0)
				s.DeliverSections(bit, []uint64{}, [][]byte{})
				return

			case <-time.After(wait):
//节流，获取任何可用的
			}
		}
//尽可能多地分配和请求服务
		sections := s.AllocateSections(bit, batch)
		request := make(chan *Retrieval)

		select {
		case <-s.quit:
//会话终止，我们无法有意义地服务，中止
			s.DeliverSections(bit, sections, make([][]byte, len(sections)))
			return

		case mux <- request:
//接受检索，必须在中止之前到达
			request <- &Retrieval{Bit: bit, Sections: sections, Context: s.ctx}

			result := <-request
			if result.Error != nil {
				s.err.Store(result.Error)
				s.Close()
			}
			s.DeliverSections(result.Bit, result.Sections, result.Bitsets)
		}
	}
}
