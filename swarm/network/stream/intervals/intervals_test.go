
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

import "testing"

//测试间隔方法添加、下一个和最后一个
//初始状态。
func Test(t *testing.T) {
	for i, tc := range []struct {
		startLimit uint64
		initial    [][2]uint64
		start      uint64
		end        uint64
		expected   string
		nextStart  uint64
		nextEnd    uint64
		last       uint64
	}{
		{
			initial:   nil,
			start:     0,
			end:       0,
			expected:  "[[0 0]]",
			nextStart: 1,
			nextEnd:   0,
			last:      0,
		},
		{
			initial:   nil,
			start:     0,
			end:       10,
			expected:  "[[0 10]]",
			nextStart: 11,
			nextEnd:   0,
			last:      10,
		},
		{
			initial:   nil,
			start:     5,
			end:       15,
			expected:  "[[5 15]]",
			nextStart: 0,
			nextEnd:   4,
			last:      15,
		},
		{
			initial:   [][2]uint64{{0, 0}},
			start:     0,
			end:       0,
			expected:  "[[0 0]]",
			nextStart: 1,
			nextEnd:   0,
			last:      0,
		},
		{
			initial:   [][2]uint64{{0, 0}},
			start:     5,
			end:       15,
			expected:  "[[0 0] [5 15]]",
			nextStart: 1,
			nextEnd:   4,
			last:      15,
		},
		{
			initial:   [][2]uint64{{5, 15}},
			start:     5,
			end:       15,
			expected:  "[[5 15]]",
			nextStart: 0,
			nextEnd:   4,
			last:      15,
		},
		{
			initial:   [][2]uint64{{5, 15}},
			start:     5,
			end:       20,
			expected:  "[[5 20]]",
			nextStart: 0,
			nextEnd:   4,
			last:      20,
		},
		{
			initial:   [][2]uint64{{5, 15}},
			start:     10,
			end:       20,
			expected:  "[[5 20]]",
			nextStart: 0,
			nextEnd:   4,
			last:      20,
		},
		{
			initial:   [][2]uint64{{5, 15}},
			start:     0,
			end:       20,
			expected:  "[[0 20]]",
			nextStart: 21,
			nextEnd:   0,
			last:      20,
		},
		{
			initial:   [][2]uint64{{5, 15}},
			start:     2,
			end:       10,
			expected:  "[[2 15]]",
			nextStart: 0,
			nextEnd:   1,
			last:      15,
		},
		{
			initial:   [][2]uint64{{5, 15}},
			start:     2,
			end:       4,
			expected:  "[[2 15]]",
			nextStart: 0,
			nextEnd:   1,
			last:      15,
		},
		{
			initial:   [][2]uint64{{5, 15}},
			start:     2,
			end:       5,
			expected:  "[[2 15]]",
			nextStart: 0,
			nextEnd:   1,
			last:      15,
		},
		{
			initial:   [][2]uint64{{5, 15}},
			start:     2,
			end:       3,
			expected:  "[[2 3] [5 15]]",
			nextStart: 0,
			nextEnd:   1,
			last:      15,
		},
		{
			initial:   [][2]uint64{{5, 15}},
			start:     2,
			end:       4,
			expected:  "[[2 15]]",
			nextStart: 0,
			nextEnd:   1,
			last:      15,
		},
		{
			initial:   [][2]uint64{{0, 1}, {5, 15}},
			start:     2,
			end:       4,
			expected:  "[[0 15]]",
			nextStart: 16,
			nextEnd:   0,
			last:      15,
		},
		{
			initial:   [][2]uint64{{0, 5}, {15, 20}},
			start:     2,
			end:       10,
			expected:  "[[0 10] [15 20]]",
			nextStart: 11,
			nextEnd:   14,
			last:      20,
		},
		{
			initial:   [][2]uint64{{0, 5}, {15, 20}},
			start:     8,
			end:       18,
			expected:  "[[0 5] [8 20]]",
			nextStart: 6,
			nextEnd:   7,
			last:      20,
		},
		{
			initial:   [][2]uint64{{0, 5}, {15, 20}},
			start:     2,
			end:       17,
			expected:  "[[0 20]]",
			nextStart: 21,
			nextEnd:   0,
			last:      20,
		},
		{
			initial:   [][2]uint64{{0, 5}, {15, 20}},
			start:     2,
			end:       25,
			expected:  "[[0 25]]",
			nextStart: 26,
			nextEnd:   0,
			last:      25,
		},
		{
			initial:   [][2]uint64{{0, 5}, {15, 20}},
			start:     5,
			end:       14,
			expected:  "[[0 20]]",
			nextStart: 21,
			nextEnd:   0,
			last:      20,
		},
		{
			initial:   [][2]uint64{{0, 5}, {15, 20}},
			start:     6,
			end:       14,
			expected:  "[[0 20]]",
			nextStart: 21,
			nextEnd:   0,
			last:      20,
		},
		{
			initial:   [][2]uint64{{0, 5}, {15, 20}, {30, 40}},
			start:     6,
			end:       29,
			expected:  "[[0 40]]",
			nextStart: 41,
			nextEnd:   0,
			last:      40,
		},
		{
			initial:   [][2]uint64{{0, 5}, {15, 20}, {30, 40}, {50, 60}},
			start:     3,
			end:       55,
			expected:  "[[0 60]]",
			nextStart: 61,
			nextEnd:   0,
			last:      60,
		},
		{
			initial:   [][2]uint64{{0, 5}, {15, 20}, {30, 40}, {50, 60}},
			start:     21,
			end:       49,
			expected:  "[[0 5] [15 60]]",
			nextStart: 6,
			nextEnd:   14,
			last:      60,
		},
		{
			initial:   [][2]uint64{{0, 5}, {15, 20}, {30, 40}, {50, 60}},
			start:     0,
			end:       100,
			expected:  "[[0 100]]",
			nextStart: 101,
			nextEnd:   0,
			last:      100,
		},
		{
			startLimit: 100,
			initial:    nil,
			start:      0,
			end:        0,
			expected:   "[]",
			nextStart:  100,
			nextEnd:    0,
			last:       0,
		},
		{
			startLimit: 100,
			initial:    nil,
			start:      20,
			end:        30,
			expected:   "[]",
			nextStart:  100,
			nextEnd:    0,
			last:       0,
		},
		{
			startLimit: 100,
			initial:    nil,
			start:      50,
			end:        100,
			expected:   "[[100 100]]",
			nextStart:  101,
			nextEnd:    0,
			last:       100,
		},
		{
			startLimit: 100,
			initial:    nil,
			start:      50,
			end:        110,
			expected:   "[[100 110]]",
			nextStart:  111,
			nextEnd:    0,
			last:       110,
		},
		{
			startLimit: 100,
			initial:    nil,
			start:      120,
			end:        130,
			expected:   "[[120 130]]",
			nextStart:  100,
			nextEnd:    119,
			last:       130,
		},
		{
			startLimit: 100,
			initial:    nil,
			start:      120,
			end:        130,
			expected:   "[[120 130]]",
			nextStart:  100,
			nextEnd:    119,
			last:       130,
		},
	} {
		intervals := NewIntervals(tc.startLimit)
		intervals.ranges = tc.initial
		intervals.Add(tc.start, tc.end)
		got := intervals.String()
		if got != tc.expected {
			t.Errorf("interval #%d: expected %s, got %s", i, tc.expected, got)
		}
		nextStart, nextEnd := intervals.Next()
		if nextStart != tc.nextStart {
			t.Errorf("interval #%d, expected next start %d, got %d", i, tc.nextStart, nextStart)
		}
		if nextEnd != tc.nextEnd {
			t.Errorf("interval #%d, expected next end %d, got %d", i, tc.nextEnd, nextEnd)
		}
		last := intervals.Last()
		if last != tc.last {
			t.Errorf("interval #%d, expected last %d, got %d", i, tc.last, last)
		}
	}
}

func TestMerge(t *testing.T) {
	for i, tc := range []struct {
		initial  [][2]uint64
		merge    [][2]uint64
		expected string
	}{
		{
			initial:  nil,
			merge:    nil,
			expected: "[]",
		},
		{
			initial:  [][2]uint64{{10, 20}},
			merge:    nil,
			expected: "[[10 20]]",
		},
		{
			initial:  nil,
			merge:    [][2]uint64{{15, 25}},
			expected: "[[15 25]]",
		},
		{
			initial:  [][2]uint64{{0, 100}},
			merge:    [][2]uint64{{150, 250}},
			expected: "[[0 100] [150 250]]",
		},
		{
			initial:  [][2]uint64{{0, 100}},
			merge:    [][2]uint64{{101, 250}},
			expected: "[[0 250]]",
		},
		{
			initial:  [][2]uint64{{0, 10}, {30, 40}},
			merge:    [][2]uint64{{20, 25}, {41, 50}},
			expected: "[[0 10] [20 25] [30 50]]",
		},
		{
			initial:  [][2]uint64{{0, 5}, {15, 20}, {30, 40}, {50, 60}},
			merge:    [][2]uint64{{6, 25}},
			expected: "[[0 25] [30 40] [50 60]]",
		},
	} {
		intervals := NewIntervals(0)
		intervals.ranges = tc.initial
		m := NewIntervals(0)
		m.ranges = tc.merge

		intervals.Merge(m)

		got := intervals.String()
		if got != tc.expected {
			t.Errorf("interval #%d: expected %s, got %s", i, tc.expected, got)
		}
	}
}
