
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

package whisperv6

import (
	"bytes"
	"testing"
	"time"
)

func TestMultipleTopicCopyInNewMessageFilter(t *testing.T) {
	w := New(nil)

	keyID, err := w.GenerateSymKey()
	if err != nil {
		t.Fatalf("Error generating symmetric key: %v", err)
	}
	api := PublicWhisperAPI{
		w:        w,
		lastUsed: make(map[string]time.Time),
	}

	t1 := [4]byte{0xde, 0xea, 0xbe, 0xef}
	t2 := [4]byte{0xca, 0xfe, 0xde, 0xca}

	crit := Criteria{
		SymKeyID: keyID,
		Topics:   []TopicType{TopicType(t1), TopicType(t2)},
	}

	_, err = api.NewMessageFilter(crit)
	if err != nil {
		t.Fatalf("Error creating the filter: %v", err)
	}

	found := false
	candidates := w.filters.getWatchersByTopic(TopicType(t1))
	for _, f := range candidates {
		if len(f.Topics) == 2 {
			if bytes.Equal(f.Topics[0], t1[:]) && bytes.Equal(f.Topics[1], t2[:]) {
				found = true
			}
		}
	}

	if !found {
		t.Fatalf("Could not find filter with both topics")
	}
}
