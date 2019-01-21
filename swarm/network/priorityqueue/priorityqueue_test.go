
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
package priorityqueue

import (
	"context"
	"sync"
	"testing"
)

func TestPriorityQueue(t *testing.T) {
	var results []string
	wg := sync.WaitGroup{}
	pq := New(3, 2)
	wg.Add(1)
	go pq.Run(context.Background(), func(v interface{}) {
		results = append(results, v.(string))
		wg.Done()
	})
	pq.Push("2.0", 2)
	wg.Wait()
	if results[0] != "2.0" {
		t.Errorf("expected first result %q, got %q", "2.0", results[0])
	}

Loop:
	for i, tc := range []struct {
		priorities []int
		values     []string
		results    []string
		errors     []error
	}{
		{
			priorities: []int{0},
			values:     []string{""},
			results:    []string{""},
		},
		{
			priorities: []int{0, 1},
			values:     []string{"0.0", "1.0"},
			results:    []string{"1.0", "0.0"},
		},
		{
			priorities: []int{1, 0},
			values:     []string{"1.0", "0.0"},
			results:    []string{"1.0", "0.0"},
		},
		{
			priorities: []int{0, 1, 1},
			values:     []string{"0.0", "1.0", "1.1"},
			results:    []string{"1.0", "1.1", "0.0"},
		},
		{
			priorities: []int{0, 0, 0},
			values:     []string{"0.0", "0.0", "0.1"},
			errors:     []error{nil, nil, ErrContention},
		},
	} {
		var results []string
		wg := sync.WaitGroup{}
		pq := New(3, 2)
		wg.Add(len(tc.values))
		for j, value := range tc.values {
			err := pq.Push(value, tc.priorities[j])
			if tc.errors != nil && err != tc.errors[j] {
				t.Errorf("expected push error %v, got %v", tc.errors[j], err)
				continue Loop
			}
			if err != nil {
				continue Loop
			}
		}
		go pq.Run(context.Background(), func(v interface{}) {
			results = append(results, v.(string))
			wg.Done()
		})
		wg.Wait()
		for k, result := range tc.results {
			if results[k] != result {
				t.Errorf("test case %v: expected %v element %q, got %q", i, k, result, results[k])
			}
		}
	}
}
