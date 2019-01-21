
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
package pot

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/swarm/log"
)

const (
	maxEachNeighbourTests = 420
	maxEachNeighbour      = 420
	maxSwap               = 420
	maxSwapTests          = 420
)

//函数（）
//log.root（）.sethandler（log.lvlfilterhandler（log.lvltrace，log.streamhandler（os.stderr，log.terminalformat（false）））
//}

type testAddr struct {
	a []byte
	i int
}

func newTestAddr(s string, i int) *testAddr {
	return &testAddr{NewAddressFromString(s), i}
}

func (a *testAddr) Address() []byte {
	return a.a
}

func (a *testAddr) String() string {
	return Label(a.a)
}

func randomTestAddr(n int, i int) *testAddr {
	v := RandomAddress().Bin()[:n]
	return newTestAddr(v, i)
}

func randomtestAddr(n int, i int) *testAddr {
	v := RandomAddress().Bin()[:n]
	return newTestAddr(v, i)
}

func indexes(t *Pot) (i []int) {
	t.Each(func(v Val) bool {
		a := v.(*testAddr)
		i = append(i, a.i)
		return true
	})
	return i
}

func testAdd(t *Pot, pof Pof, j int, values ...string) (_ *Pot, n int, f bool) {
	for i, val := range values {
		t, n, f = Add(t, newTestAddr(val, i+j), pof)
	}
	return t, n, f
}

//从罐中移除不存在的元素
func TestPotRemoveNonExisting(t *testing.T) {
	pof := DefaultPof(8)
	n := NewPot(newTestAddr("00111100", 0), 0)
	n, _, _ = Remove(n, newTestAddr("00000101", 0), pof)
	exp := "00111100"
	got := Label(n.Pin())
	if got[:8] != exp {
		t.Fatalf("incorrect pinned value. Expected %v, got %v", exp, got[:8])
	}
}

//此测试创建分层的pot树，因此任何子节点都将
//child_po=父级_po+1。
//然后从树的中间删除一个节点。
func TestPotRemoveSameBin(t *testing.T) {
	pof := DefaultPof(8)
	n := NewPot(newTestAddr("11111111", 0), 0)
	n, _, _ = testAdd(n, pof, 1, "00000000", "01000000", "01100000", "01110000", "01111000")
	n, _, _ = Remove(n, newTestAddr("01110000", 0), pof)
	inds := indexes(n)
	goti := n.Size()
	expi := 5
	if goti != expi {
		t.Fatalf("incorrect number of elements in Pot. Expected %v, got %v", expi, goti)
	}
	inds = indexes(n)
	got := fmt.Sprintf("%v", inds)
	exp := "[5 3 2 1 0]"
	if got != exp {
		t.Fatalf("incorrect indexes in iteration over Pot. Expected %v, got %v", exp, got)
	}
}

//这个测试创建了一个平壶树（所有元素都是一根叶子）。
//因此他们都有相同的采购订单。
//然后从容器中移除任意元素。
func TestPotRemoveDifferentBins(t *testing.T) {
	pof := DefaultPof(8)
	n := NewPot(newTestAddr("11111111", 0), 0)
	n, _, _ = testAdd(n, pof, 1, "00000000", "10000000", "11000000", "11100000", "11110000")
	n, _, _ = Remove(n, newTestAddr("11100000", 0), pof)
	inds := indexes(n)
	goti := n.Size()
	expi := 5
	if goti != expi {
		t.Fatalf("incorrect number of elements in Pot. Expected %v, got %v", expi, goti)
	}
	inds = indexes(n)
	got := fmt.Sprintf("%v", inds)
	exp := "[1 2 3 5 0]"
	if got != exp {
		t.Fatalf("incorrect indexes in iteration over Pot. Expected %v, got %v", exp, got)
	}
	n, _, _ = testAdd(n, pof, 4, "11100000")
	inds = indexes(n)
	got = fmt.Sprintf("%v", inds)
	exp = "[1 2 3 4 5 0]"
	if got != exp {
		t.Fatalf("incorrect indexes in iteration over Pot. Expected %v, got %v", exp, got)
	}
}

func TestPotAdd(t *testing.T) {
	pof := DefaultPof(8)
	n := NewPot(newTestAddr("00111100", 0), 0)
//销设置正确
	exp := "00111100"
	got := Label(n.Pin())[:8]
	if got != exp {
		t.Fatalf("incorrect pinned value. Expected %v, got %v", exp, got)
	}
//检查尺寸
	goti := n.Size()
	expi := 1
	if goti != expi {
		t.Fatalf("incorrect number of elements in Pot. Expected %v, got %v", expi, goti)
	}

	n, _, _ = testAdd(n, pof, 1, "01111100", "00111100", "01111100", "00011100")
//检查尺寸
	goti = n.Size()
	expi = 3
	if goti != expi {
		t.Fatalf("incorrect number of elements in Pot. Expected %v, got %v", expi, goti)
	}
	inds := indexes(n)
	got = fmt.Sprintf("%v", inds)
	exp = "[3 4 2]"
	if got != exp {
		t.Fatalf("incorrect indexes in iteration over Pot. Expected %v, got %v", exp, got)
	}
}

func TestPotRemove(t *testing.T) {
	pof := DefaultPof(8)
	n := NewPot(newTestAddr("00111100", 0), 0)
	n, _, _ = Remove(n, newTestAddr("00111100", 0), pof)
	exp := "<nil>"
	got := Label(n.Pin())
	if got != exp {
		t.Fatalf("incorrect pinned value. Expected %v, got %v", exp, got)
	}
	n, _, _ = testAdd(n, pof, 1, "00000000", "01111100", "00111100", "00011100")
	n, _, _ = Remove(n, newTestAddr("00111100", 0), pof)
	goti := n.Size()
	expi := 3
	if goti != expi {
		t.Fatalf("incorrect number of elements in Pot. Expected %v, got %v", expi, goti)
	}
	inds := indexes(n)
	got = fmt.Sprintf("%v", inds)
	exp = "[2 4 1]"
	if got != exp {
		t.Fatalf("incorrect indexes in iteration over Pot. Expected %v, got %v", exp, got)
	}
n, _, _ = Remove(n, newTestAddr("00111100", 0), pof) //再次删除相同的元素
	inds = indexes(n)
	got = fmt.Sprintf("%v", inds)
	if got != exp {
		t.Fatalf("incorrect indexes in iteration over Pot. Expected %v, got %v", exp, got)
	}
n, _, _ = Remove(n, newTestAddr("00000000", 0), pof) //移除第一个元素
	inds = indexes(n)
	got = fmt.Sprintf("%v", inds)
	exp = "[2 4]"
	if got != exp {
		t.Fatalf("incorrect indexes in iteration over Pot. Expected %v, got %v", exp, got)
	}
}

func TestPotSwap(t *testing.T) {
	for i := 0; i < maxSwapTests; i++ {
		alen := maxkeylen
		pof := DefaultPof(alen)
		max := rand.Intn(maxSwap)

		n := NewPot(nil, 0)
		var m []*testAddr
		var found bool
		for j := 0; j < 2*max; {
			v := randomtestAddr(alen, j)
			n, _, found = Add(n, v, pof)
			if !found {
				m = append(m, v)
				j++
			}
		}
		k := make(map[string]*testAddr)
		for j := 0; j < max; {
			v := randomtestAddr(alen, 1)
			_, found := k[Label(v)]
			if !found {
				k[Label(v)] = v
				j++
			}
		}
		for _, v := range k {
			m = append(m, v)
		}
		f := func(v Val) Val {
			tv := v.(*testAddr)
			if tv.i < max {
				return nil
			}
			tv.i = 0
			return v
		}
		for _, val := range m {
			n, _, _, _ = Swap(n, val, pof, func(v Val) Val {
				if v == nil {
					return val
				}
				return f(v)
			})
		}
		sum := 0
		n.Each(func(v Val) bool {
			if v == nil {
				return true
			}
			sum++
			tv := v.(*testAddr)
			if tv.i > 1 {
				t.Fatalf("item value incorrect, expected 0, got %v", tv.i)
			}
			return true
		})
		if sum != 2*max {
			t.Fatalf("incorrect number of elements. expected %v, got %v", 2*max, sum)
		}
		if sum != n.Size() {
			t.Fatalf("incorrect size. expected %v, got %v", sum, n.Size())
		}
	}
}

func checkPo(val Val, pof Pof) func(Val, int) error {
	return func(v Val, po int) error {
//检查订单
		exp, _ := pof(val, v, 0)
		if po != exp {
			return fmt.Errorf("incorrect prox order for item %v in neighbour iteration for %v. Expected %v, got %v", v, val, exp, po)
		}
		return nil
	}
}

func checkOrder(val Val) func(Val, int) error {
	po := maxkeylen
	return func(v Val, p int) error {
		if po < p {
			return fmt.Errorf("incorrect order for item %v in neighbour iteration for %v. PO %v > %v (previous max)", v, val, p, po)
		}
		po = p
		return nil
	}
}

func checkValues(m map[string]bool, val Val) func(Val, int) error {
	return func(v Val, po int) error {
		duplicate, ok := m[Label(v)]
		if !ok {
			return fmt.Errorf("alien value %v", v)
		}
		if duplicate {
			return fmt.Errorf("duplicate value returned: %v", v)
		}
		m[Label(v)] = true
		return nil
	}
}

var errNoCount = errors.New("not count")

func testPotEachNeighbour(n *Pot, pof Pof, val Val, expCount int, fs ...func(Val, int) error) error {
	var err error
	var count int
	n.EachNeighbour(val, pof, func(v Val, po int) bool {
		for _, f := range fs {
			err = f(v, po)
			if err != nil {
				return err.Error() == errNoCount.Error()
			}
		}
		count++
		return count != expCount
	})
	if err == nil && count < expCount {
		return fmt.Errorf("not enough neighbours returned, expected %v, got %v", expCount, count)
	}
	return err
}

const (
	mergeTestCount  = 5
	mergeTestChoose = 5
)

func TestPotMergeCommon(t *testing.T) {
	vs := make([]*testAddr, mergeTestCount)
	for i := 0; i < maxEachNeighbourTests; i++ {
		alen := maxkeylen
		pof := DefaultPof(alen)

		for j := 0; j < len(vs); j++ {
			vs[j] = randomtestAddr(alen, j)
		}
		max0 := rand.Intn(mergeTestChoose) + 1
		max1 := rand.Intn(mergeTestChoose) + 1
		n0 := NewPot(nil, 0)
		n1 := NewPot(nil, 0)
		log.Trace(fmt.Sprintf("round %v: %v - %v", i, max0, max1))
		m := make(map[string]bool)
		var found bool
		for j := 0; j < max0; {
			r := rand.Intn(max0)
			v := vs[r]
			n0, _, found = Add(n0, v, pof)
			if !found {
				m[Label(v)] = false
				j++
			}
		}
		expAdded := 0

		for j := 0; j < max1; {
			r := rand.Intn(max1)
			v := vs[r]
			n1, _, found = Add(n1, v, pof)
			if !found {
				j++
			}
			_, found = m[Label(v)]
			if !found {
				expAdded++
				m[Label(v)] = false
			}
		}
		if i < 6 {
			continue
		}
		expSize := len(m)
		log.Trace(fmt.Sprintf("%v-0: pin: %v, size: %v", i, n0.Pin(), max0))
		log.Trace(fmt.Sprintf("%v-1: pin: %v, size: %v", i, n1.Pin(), max1))
		log.Trace(fmt.Sprintf("%v: merged tree size: %v, newly added: %v", i, expSize, expAdded))
		n, common := Union(n0, n1, pof)
		added := n1.Size() - common
		size := n.Size()

		if expSize != size {
			t.Fatalf("%v: incorrect number of elements in merged pot, expected %v, got %v\n%v", i, expSize, size, n)
		}
		if expAdded != added {
			t.Fatalf("%v: incorrect number of added elements in merged pot, expected %v, got %v", i, expAdded, added)
		}
		if !checkDuplicates(n) {
			t.Fatalf("%v: merged pot contains duplicates: \n%v", i, n)
		}
		for k := range m {
			_, _, found = Add(n, newTestAddr(k, 0), pof)
			if !found {
				t.Fatalf("%v: merged pot (size:%v, added: %v) missing element %v", i, size, added, k)
			}
		}
	}
}

func TestPotMergeScale(t *testing.T) {
	for i := 0; i < maxEachNeighbourTests; i++ {
		alen := maxkeylen
		pof := DefaultPof(alen)
		max0 := rand.Intn(maxEachNeighbour) + 1
		max1 := rand.Intn(maxEachNeighbour) + 1
		n0 := NewPot(nil, 0)
		n1 := NewPot(nil, 0)
		log.Trace(fmt.Sprintf("round %v: %v - %v", i, max0, max1))
		m := make(map[string]bool)
		var found bool
		for j := 0; j < max0; {
			v := randomtestAddr(alen, j)
			n0, _, found = Add(n0, v, pof)
			if !found {
				m[Label(v)] = false
				j++
			}
		}
		expAdded := 0

		for j := 0; j < max1; {
			v := randomtestAddr(alen, j)
			n1, _, found = Add(n1, v, pof)
			if !found {
				j++
			}
			_, found = m[Label(v)]
			if !found {
				expAdded++
				m[Label(v)] = false
			}
		}
		if i < 6 {
			continue
		}
		expSize := len(m)
		log.Trace(fmt.Sprintf("%v-0: pin: %v, size: %v", i, n0.Pin(), max0))
		log.Trace(fmt.Sprintf("%v-1: pin: %v, size: %v", i, n1.Pin(), max1))
		log.Trace(fmt.Sprintf("%v: merged tree size: %v, newly added: %v", i, expSize, expAdded))
		n, common := Union(n0, n1, pof)
		added := n1.Size() - common
		size := n.Size()

		if expSize != size {
			t.Fatalf("%v: incorrect number of elements in merged pot, expected %v, got %v", i, expSize, size)
		}
		if expAdded != added {
			t.Fatalf("%v: incorrect number of added elements in merged pot, expected %v, got %v", i, expAdded, added)
		}
		if !checkDuplicates(n) {
			t.Fatalf("%v: merged pot contains duplicates", i)
		}
		for k := range m {
			_, _, found = Add(n, newTestAddr(k, 0), pof)
			if !found {
				t.Fatalf("%v: merged pot (size:%v, added: %v) missing element %v", i, size, added, k)
			}
		}
	}
}

func checkDuplicates(t *Pot) bool {
	po := -1
	for _, c := range t.bins {
		if c == nil {
			return false
		}
		if c.po <= po || !checkDuplicates(c) {
			return false
		}
		po = c.po
	}
	return true
}

func TestPotEachNeighbourSync(t *testing.T) {
	for i := 0; i < maxEachNeighbourTests; i++ {
		alen := maxkeylen
		pof := DefaultPof(maxkeylen)
		max := rand.Intn(maxEachNeighbour/2) + maxEachNeighbour/2
		pin := randomTestAddr(alen, 0)
		n := NewPot(pin, 0)
		m := make(map[string]bool)
		m[Label(pin)] = false
		for j := 1; j <= max; j++ {
			v := randomTestAddr(alen, j)
			n, _, _ = Add(n, v, pof)
			m[Label(v)] = false
		}

		size := n.Size()
		if size < 2 {
			continue
		}
		count := rand.Intn(size/2) + size/2
		val := randomTestAddr(alen, max+1)
		log.Trace(fmt.Sprintf("%v: pin: %v, size: %v, val: %v, count: %v", i, n.Pin(), size, val, count))
		err := testPotEachNeighbour(n, pof, val, count, checkPo(val, pof), checkOrder(val), checkValues(m, val))
		if err != nil {
			t.Fatal(err)
		}
		minPoFound := alen
		maxPoNotFound := 0
		for k, found := range m {
			po, _ := pof(val, newTestAddr(k, 0), 0)
			if found {
				if po < minPoFound {
					minPoFound = po
				}
			} else {
				if po > maxPoNotFound {
					maxPoNotFound = po
				}
			}
		}
		if minPoFound < maxPoNotFound {
			t.Fatalf("incorrect neighbours returned: found one with PO %v < there was one not found with PO %v", minPoFound, maxPoNotFound)
		}
	}
}

func TestPotEachNeighbourAsync(t *testing.T) {
	for i := 0; i < maxEachNeighbourTests; i++ {
		max := rand.Intn(maxEachNeighbour/2) + maxEachNeighbour/2
		alen := maxkeylen
		pof := DefaultPof(alen)
		n := NewPot(randomTestAddr(alen, 0), 0)
		size := 1
		var found bool
		for j := 1; j <= max; j++ {
			v := randomTestAddr(alen, j)
			n, _, found = Add(n, v, pof)
			if !found {
				size++
			}
		}
		if size != n.Size() {
			t.Fatal(n)
		}
		if size < 2 {
			continue
		}
		count := rand.Intn(size/2) + size/2
		val := randomTestAddr(alen, max+1)

		mu := sync.Mutex{}
		m := make(map[string]bool)
		maxPos := rand.Intn(alen)
		log.Trace(fmt.Sprintf("%v: pin: %v, size: %v, val: %v, count: %v, maxPos: %v", i, n.Pin(), size, val, count, maxPos))
		msize := 0
		remember := func(v Val, po int) error {
			if po > maxPos {
				return errNoCount
			}
			m[Label(v)] = true
			msize++
			return nil
		}
		if i == 0 {
			continue
		}
		testPotEachNeighbour(n, pof, val, count, remember)
		d := 0
		forget := func(v Val, po int) {
			mu.Lock()
			defer mu.Unlock()
			d++
			delete(m, Label(v))
		}

		n.EachNeighbourAsync(val, pof, count, maxPos, forget, true)
		if d != msize {
			t.Fatalf("incorrect number of neighbour calls in async iterator. expected %v, got %v", msize, d)
		}
		if len(m) != 0 {
			t.Fatalf("incorrect neighbour calls in async iterator. %v items missed:\n%v", len(m), n)
		}
	}
}

func benchmarkEachNeighbourSync(t *testing.B, max, count int, d time.Duration) {
	t.ReportAllocs()
	alen := maxkeylen
	pof := DefaultPof(alen)
	pin := randomTestAddr(alen, 0)
	n := NewPot(pin, 0)
	var found bool
	for j := 1; j <= max; {
		v := randomTestAddr(alen, j)
		n, _, found = Add(n, v, pof)
		if !found {
			j++
		}
	}
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		val := randomTestAddr(alen, max+1)
		m := 0
		n.EachNeighbour(val, pof, func(v Val, po int) bool {
			time.Sleep(d)
			m++
			return m != count
		})
	}
	t.StopTimer()
	stats := new(runtime.MemStats)
	runtime.ReadMemStats(stats)
}

func benchmarkEachNeighbourAsync(t *testing.B, max, count int, d time.Duration) {
	t.ReportAllocs()
	alen := maxkeylen
	pof := DefaultPof(alen)
	pin := randomTestAddr(alen, 0)
	n := NewPot(pin, 0)
	var found bool
	for j := 1; j <= max; {
		v := randomTestAddr(alen, j)
		n, _, found = Add(n, v, pof)
		if !found {
			j++
		}
	}
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		val := randomTestAddr(alen, max+1)
		n.EachNeighbourAsync(val, pof, count, alen, func(v Val, po int) {
			time.Sleep(d)
		}, true)
	}
	t.StopTimer()
	stats := new(runtime.MemStats)
	runtime.ReadMemStats(stats)
}

func BenchmarkEachNeighbourSync_3_1_0(t *testing.B) {
	benchmarkEachNeighbourSync(t, 1000, 10, 1*time.Microsecond)
}
func BenchmarkEachNeighboursAsync_3_1_0(t *testing.B) {
	benchmarkEachNeighbourAsync(t, 1000, 10, 1*time.Microsecond)
}
func BenchmarkEachNeighbourSync_3_2_0(t *testing.B) {
	benchmarkEachNeighbourSync(t, 1000, 100, 1*time.Microsecond)
}
func BenchmarkEachNeighboursAsync_3_2_0(t *testing.B) {
	benchmarkEachNeighbourAsync(t, 1000, 100, 1*time.Microsecond)
}
func BenchmarkEachNeighbourSync_3_3_0(t *testing.B) {
	benchmarkEachNeighbourSync(t, 1000, 1000, 1*time.Microsecond)
}
func BenchmarkEachNeighboursAsync_3_3_0(t *testing.B) {
	benchmarkEachNeighbourAsync(t, 1000, 1000, 1*time.Microsecond)
}

func BenchmarkEachNeighbourSync_3_1_1(t *testing.B) {
	benchmarkEachNeighbourSync(t, 1000, 10, 2*time.Microsecond)
}
func BenchmarkEachNeighboursAsync_3_1_1(t *testing.B) {
	benchmarkEachNeighbourAsync(t, 1000, 10, 2*time.Microsecond)
}
func BenchmarkEachNeighbourSync_3_2_1(t *testing.B) {
	benchmarkEachNeighbourSync(t, 1000, 100, 2*time.Microsecond)
}
func BenchmarkEachNeighboursAsync_3_2_1(t *testing.B) {
	benchmarkEachNeighbourAsync(t, 1000, 100, 2*time.Microsecond)
}
func BenchmarkEachNeighbourSync_3_3_1(t *testing.B) {
	benchmarkEachNeighbourSync(t, 1000, 1000, 2*time.Microsecond)
}
func BenchmarkEachNeighboursAsync_3_3_1(t *testing.B) {
	benchmarkEachNeighbourAsync(t, 1000, 1000, 2*time.Microsecond)
}

func BenchmarkEachNeighbourSync_3_1_2(t *testing.B) {
	benchmarkEachNeighbourSync(t, 1000, 10, 4*time.Microsecond)
}
func BenchmarkEachNeighboursAsync_3_1_2(t *testing.B) {
	benchmarkEachNeighbourAsync(t, 1000, 10, 4*time.Microsecond)
}
func BenchmarkEachNeighbourSync_3_2_2(t *testing.B) {
	benchmarkEachNeighbourSync(t, 1000, 100, 4*time.Microsecond)
}
func BenchmarkEachNeighboursAsync_3_2_2(t *testing.B) {
	benchmarkEachNeighbourAsync(t, 1000, 100, 4*time.Microsecond)
}
func BenchmarkEachNeighbourSync_3_3_2(t *testing.B) {
	benchmarkEachNeighbourSync(t, 1000, 1000, 4*time.Microsecond)
}
func BenchmarkEachNeighboursAsync_3_3_2(t *testing.B) {
	benchmarkEachNeighbourAsync(t, 1000, 1000, 4*time.Microsecond)
}

func BenchmarkEachNeighbourSync_3_1_3(t *testing.B) {
	benchmarkEachNeighbourSync(t, 1000, 10, 8*time.Microsecond)
}
func BenchmarkEachNeighboursAsync_3_1_3(t *testing.B) {
	benchmarkEachNeighbourAsync(t, 1000, 10, 8*time.Microsecond)
}
func BenchmarkEachNeighbourSync_3_2_3(t *testing.B) {
	benchmarkEachNeighbourSync(t, 1000, 100, 8*time.Microsecond)
}
func BenchmarkEachNeighboursAsync_3_2_3(t *testing.B) {
	benchmarkEachNeighbourAsync(t, 1000, 100, 8*time.Microsecond)
}
func BenchmarkEachNeighbourSync_3_3_3(t *testing.B) {
	benchmarkEachNeighbourSync(t, 1000, 1000, 8*time.Microsecond)
}
func BenchmarkEachNeighboursAsync_3_3_3(t *testing.B) {
	benchmarkEachNeighbourAsync(t, 1000, 1000, 8*time.Microsecond)
}

func BenchmarkEachNeighbourSync_3_1_4(t *testing.B) {
	benchmarkEachNeighbourSync(t, 1000, 10, 16*time.Microsecond)
}
func BenchmarkEachNeighboursAsync_3_1_4(t *testing.B) {
	benchmarkEachNeighbourAsync(t, 1000, 10, 16*time.Microsecond)
}
func BenchmarkEachNeighbourSync_3_2_4(t *testing.B) {
	benchmarkEachNeighbourSync(t, 1000, 100, 16*time.Microsecond)
}
func BenchmarkEachNeighboursAsync_3_2_4(t *testing.B) {
	benchmarkEachNeighbourAsync(t, 1000, 100, 16*time.Microsecond)
}
func BenchmarkEachNeighbourSync_3_3_4(t *testing.B) {
	benchmarkEachNeighbourSync(t, 1000, 1000, 16*time.Microsecond)
}
func BenchmarkEachNeighboursAsync_3_3_4(t *testing.B) {
	benchmarkEachNeighbourAsync(t, 1000, 1000, 16*time.Microsecond)
}
