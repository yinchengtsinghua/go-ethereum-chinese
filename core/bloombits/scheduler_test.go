
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
	"math/big"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

//测试调度程序可以对检索请求进行重复数据消除和转发
//底层的获取器和服务响应返回，与并发无关
//请求客户机或服务数据获取程序。
func TestSchedulerSingleClientSingleFetcher(t *testing.T) { testScheduler(t, 1, 1, 5000) }
func TestSchedulerSingleClientMultiFetcher(t *testing.T)  { testScheduler(t, 1, 10, 5000) }
func TestSchedulerMultiClientSingleFetcher(t *testing.T)  { testScheduler(t, 10, 1, 5000) }
func TestSchedulerMultiClientMultiFetcher(t *testing.T)   { testScheduler(t, 10, 10, 5000) }

func testScheduler(t *testing.T, clients int, fetchers int, requests int) {
	f := newScheduler(0)

//创建一批处理程序goroutine，以响应bloom位请求和
//把它们交给调度。
	var fetchPend sync.WaitGroup
	fetchPend.Add(fetchers)
	defer fetchPend.Wait()

	fetch := make(chan *request, 16)
	defer close(fetch)

	var delivered uint32
	for i := 0; i < fetchers; i++ {
		go func() {
			defer fetchPend.Done()

			for req := range fetch {
				time.Sleep(time.Duration(rand.Intn(int(100 * time.Microsecond))))
				atomic.AddUint32(&delivered, 1)

				f.deliver([]uint64{
req.section + uint64(requests), //未请求的数据（确保不超出界限）
req.section,                    //请求数据
req.section,                    //重复的数据（确保不会双重关闭任何内容）
				}, [][]byte{
					{},
					new(big.Int).SetUint64(req.section).Bytes(),
					new(big.Int).SetUint64(req.section).Bytes(),
				})
			}
		}()
	}
//启动一批goroutine以同时运行计划任务
	quit := make(chan struct{})

	var pend sync.WaitGroup
	pend.Add(clients)

	for i := 0; i < clients; i++ {
		go func() {
			defer pend.Done()

			in := make(chan uint64, 16)
			out := make(chan []byte, 16)

			f.run(in, fetch, out, quit, &pend)

			go func() {
				for j := 0; j < requests; j++ {
					in <- uint64(j)
				}
				close(in)
			}()

			for j := 0; j < requests; j++ {
				bits := <-out
				if want := new(big.Int).SetUint64(uint64(j)).Bytes(); !bytes.Equal(bits, want) {
					t.Errorf("vector %d: delivered content mismatch: have %x, want %x", j, bits, want)
				}
			}
		}()
	}
	pend.Wait()

	if have := atomic.LoadUint32(&delivered); int(have) != requests {
		t.Errorf("request count mismatch: have %v, want %v", have, requests)
	}
}
