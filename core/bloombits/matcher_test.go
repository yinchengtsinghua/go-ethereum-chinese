
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
	"context"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

const testSectionSize = 4096

//测试通配符筛选规则（nil）是否可以指定并且处理得很好。
func TestMatcherWildcards(t *testing.T) {
	matcher := NewMatcher(testSectionSize, [][][]byte{
{common.Address{}.Bytes(), common.Address{0x01}.Bytes()}, //默认地址不是通配符
{common.Hash{}.Bytes(), common.Hash{0x01}.Bytes()},       //默认哈希不是通配符
{common.Hash{0x01}.Bytes()},                              //简单规则，健全检查
{common.Hash{0x01}.Bytes(), nil},                         //通配符后缀，删除规则
{nil, common.Hash{0x01}.Bytes()},                         //通配符前缀，删除规则
{nil, nil},                                               //通配符组合，删除规则
{},                                                       //初始化通配符规则，删除规则
nil,                                                      //适当的通配符规则，删除规则
	})
	if len(matcher.filters) != 3 {
		t.Fatalf("filter system size mismatch: have %d, want %d", len(matcher.filters), 3)
	}
	if len(matcher.filters[0]) != 2 {
		t.Fatalf("address clause size mismatch: have %d, want %d", len(matcher.filters[0]), 2)
	}
	if len(matcher.filters[1]) != 2 {
		t.Fatalf("combo topic clause size mismatch: have %d, want %d", len(matcher.filters[1]), 2)
	}
	if len(matcher.filters[2]) != 1 {
		t.Fatalf("singletone topic clause size mismatch: have %d, want %d", len(matcher.filters[2]), 1)
	}
}

//在单个连续工作流上测试匹配器管道，而不中断。
func TestMatcherContinuous(t *testing.T) {
	testMatcherDiffBatches(t, [][]bloomIndexes{{{10, 20, 30}}}, 0, 100000, false, 75)
	testMatcherDiffBatches(t, [][]bloomIndexes{{{32, 3125, 100}}, {{40, 50, 10}}}, 0, 100000, false, 81)
	testMatcherDiffBatches(t, [][]bloomIndexes{{{4, 8, 11}, {7, 8, 17}}, {{9, 9, 12}, {15, 20, 13}}, {{18, 15, 15}, {12, 10, 4}}}, 0, 10000, false, 36)
}

//在不断中断和恢复的工作模式下测试匹配器管道
//为了确保数据项只被请求一次。
func TestMatcherIntermittent(t *testing.T) {
	testMatcherDiffBatches(t, [][]bloomIndexes{{{10, 20, 30}}}, 0, 100000, true, 75)
	testMatcherDiffBatches(t, [][]bloomIndexes{{{32, 3125, 100}}, {{40, 50, 10}}}, 0, 100000, true, 81)
	testMatcherDiffBatches(t, [][]bloomIndexes{{{4, 8, 11}, {7, 8, 17}}, {{9, 9, 12}, {15, 20, 13}}, {{18, 15, 15}, {12, 10, 4}}}, 0, 10000, true, 36)
}

//在随机输入上测试匹配器管道，以期捕获异常。
func TestMatcherRandom(t *testing.T) {
	for i := 0; i < 10; i++ {
		testMatcherBothModes(t, makeRandomIndexes([]int{1}, 50), 0, 10000, 0)
		testMatcherBothModes(t, makeRandomIndexes([]int{3}, 50), 0, 10000, 0)
		testMatcherBothModes(t, makeRandomIndexes([]int{2, 2, 2}, 20), 0, 10000, 0)
		testMatcherBothModes(t, makeRandomIndexes([]int{5, 5, 5}, 50), 0, 10000, 0)
		testMatcherBothModes(t, makeRandomIndexes([]int{4, 4, 4}, 20), 0, 10000, 0)
	}
}

//如果起始块是
//从8的倍数换档。这需要包括优化
//位集匹配https://github.com/ethereum/go-ethereum/issues/15309。
func TestMatcherShifted(t *testing.T) {
//块0在测试中始终匹配，跳过前8个块
//开始在匹配器位集中获取一个可能的零字节。

//要保持第二个位集字节为零，筛选器必须只与第一个匹配
//块16中的时间，所以做一个全16位的过滤器就足够了。

//为了使起始块不被8整除，块号9是第一个
//这将引入移位，而不匹配块0。
	testMatcherBothModes(t, [][]bloomIndexes{{{16, 16, 16}}}, 9, 64, 0)
}

//所有匹配的测试不会崩溃（内部特殊情况）。
func TestWildcardMatcher(t *testing.T) {
	testMatcherBothModes(t, nil, 0, 10000, 0)
}

//makerandomndexes生成一个由多个过滤器组成的随机过滤器系统。
//标准，每个都有一个地址和任意的Bloom列表组件
//许多主题Bloom列出组件。
func makeRandomIndexes(lengths []int, max int) [][]bloomIndexes {
	res := make([][]bloomIndexes, len(lengths))
	for i, topics := range lengths {
		res[i] = make([]bloomIndexes, topics)
		for j := 0; j < topics; j++ {
			for k := 0; k < len(res[i][j]); k++ {
				res[i][j][k] = uint(rand.Intn(max-1) + 2)
			}
		}
	}
	return res
}

//testmacherdiffbatches在单个传递中运行给定的匹配测试，并且
//在批量交付模式下，验证是否处理了所有类型的交付。
//正确无误。
func testMatcherDiffBatches(t *testing.T, filter [][]bloomIndexes, start, blocks uint64, intermittent bool, retrievals uint32) {
	singleton := testMatcher(t, filter, start, blocks, intermittent, retrievals, 1)
	batched := testMatcher(t, filter, start, blocks, intermittent, retrievals, 16)

	if singleton != batched {
		t.Errorf("filter = %v blocks = %v intermittent = %v: request count mismatch, %v in signleton vs. %v in batched mode", filter, blocks, intermittent, singleton, batched)
	}
}

//TestMatcherBothModes在连续和
//在间歇模式下，验证请求计数是否匹配。
func testMatcherBothModes(t *testing.T, filter [][]bloomIndexes, start, blocks uint64, retrievals uint32) {
	continuous := testMatcher(t, filter, start, blocks, false, retrievals, 16)
	intermittent := testMatcher(t, filter, start, blocks, true, retrievals, 16)

	if continuous != intermittent {
		t.Errorf("filter = %v blocks = %v: request count mismatch, %v in continuous vs. %v in intermittent mode", filter, blocks, continuous, intermittent)
	}
}

//TestMatcher是一个通用测试程序，用于运行给定的Matcher测试并返回
//在不同模式之间进行交叉验证的请求数。
func testMatcher(t *testing.T, filter [][]bloomIndexes, start, blocks uint64, intermittent bool, retrievals uint32, maxReqCount int) uint32 {
//创建一个新的匹配器，模拟显式随机位集
	matcher := NewMatcher(testSectionSize, nil)
	matcher.filters = filter

	for _, rule := range filter {
		for _, topic := range rule {
			for _, bit := range topic {
				matcher.addScheduler(bit)
			}
		}
	}
//跟踪发出的检索请求数
	var requested uint32

//启动筛选器和检索器goroutine的匹配会话
	quit := make(chan struct{})
	matches := make(chan uint64, 16)

	session, err := matcher.Start(context.Background(), start, blocks-1, matches)
	if err != nil {
		t.Fatalf("failed to stat matcher session: %v", err)
	}
	startRetrievers(session, quit, &requested, maxReqCount)

//遍历所有块并验证管道是否生成正确的匹配项
	for i := start; i < blocks; i++ {
		if expMatch3(filter, i) {
			match, ok := <-matches
			if !ok {
				t.Errorf("filter = %v  blocks = %v  intermittent = %v: expected #%v, results channel closed", filter, blocks, intermittent, i)
				return 0
			}
			if match != i {
				t.Errorf("filter = %v  blocks = %v  intermittent = %v: expected #%v, got #%v", filter, blocks, intermittent, i, match)
			}
//如果我们正在测试间歇模式，请中止并重新启动管道。
			if intermittent {
				session.Close()
				close(quit)

				quit = make(chan struct{})
				matches = make(chan uint64, 16)

				session, err = matcher.Start(context.Background(), i+1, blocks-1, matches)
				if err != nil {
					t.Fatalf("failed to stat matcher session: %v", err)
				}
				startRetrievers(session, quit, &requested, maxReqCount)
			}
		}
	}
//确保结果通道在最后一个块后被拆除
	match, ok := <-matches
	if ok {
		t.Errorf("filter = %v  blocks = %v  intermittent = %v: expected closed channel, got #%v", filter, blocks, intermittent, match)
	}
//清理会话并确保与预期的检索计数匹配
	session.Close()
	close(quit)

	if retrievals != 0 && requested != retrievals {
		t.Errorf("filter = %v  blocks = %v  intermittent = %v: request count mismatch, have #%v, want #%v", filter, blocks, intermittent, requested, retrievals)
	}
	return requested
}

//StartRetriever启动一批Goroutines，用于侦听节请求
//为他们服务。
func startRetrievers(session *MatcherSession, quit chan struct{}, retrievals *uint32, batch int) {
	requests := make(chan chan *Retrieval)

	for i := 0; i < 10; i++ {
//启动多路复用器以测试多线程执行
		go session.Multiplex(batch, 100*time.Microsecond, requests)

//启动与上述多路复用器匹配的服务
		go func() {
			for {
//等待服务请求或关闭
				select {
				case <-quit:
					return

				case request := <-requests:
					task := <-request

					task.Bitsets = make([][]byte, len(task.Sections))
					for i, section := range task.Sections {
if rand.Int()%4 != 0 { //处理偶尔丢失的交货
							task.Bitsets[i] = generateBitset(task.Bit, section)
							atomic.AddUint32(retrievals, 1)
						}
					}
					request <- task
				}
			}
		}()
	}
}

//generateBitset为给定的bloom位和节生成旋转的位集
//数字。
func generateBitset(bit uint, section uint64) []byte {
	bitset := make([]byte, testSectionSize/8)
	for i := 0; i < len(bitset); i++ {
		for b := 0; b < 8; b++ {
			blockIdx := section*testSectionSize + uint64(i*8+b)
			bitset[i] += bitset[i]
			if (blockIdx % uint64(bit)) == 0 {
				bitset[i]++
			}
		}
	}
	return bitset
}

func expMatch1(filter bloomIndexes, i uint64) bool {
	for _, ii := range filter {
		if (i % uint64(ii)) != 0 {
			return false
		}
	}
	return true
}

func expMatch2(filter []bloomIndexes, i uint64) bool {
	for _, ii := range filter {
		if expMatch1(ii, i) {
			return true
		}
	}
	return false
}

func expMatch3(filter [][]bloomIndexes, i uint64) bool {
	for _, ii := range filter {
		if !expMatch2(ii, i) {
			return false
		}
	}
	return true
}
