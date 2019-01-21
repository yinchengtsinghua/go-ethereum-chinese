
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

//包块哈希的内存存储层

package storage

import (
	"context"

	lru "github.com/hashicorp/golang-lru"
)

type MemStore struct {
	cache    *lru.Cache
	disabled bool
}

//newmemstore正在实例化memstore缓存，以保留所有经常请求的缓存
//“cache”lru缓存中的块。
func NewMemStore(params *StoreParams, _ *LDBStore) (m *MemStore) {
	if params.CacheCapacity == 0 {
		return &MemStore{
			disabled: true,
		}
	}

	c, err := lru.New(int(params.CacheCapacity))
	if err != nil {
		panic(err)
	}

	return &MemStore{
		cache: c,
	}
}

func (m *MemStore) Get(_ context.Context, addr Address) (Chunk, error) {
	if m.disabled {
		return nil, ErrChunkNotFound
	}

	c, ok := m.cache.Get(string(addr))
	if !ok {
		return nil, ErrChunkNotFound
	}
	return c.(Chunk), nil
}

func (m *MemStore) Put(_ context.Context, c Chunk) error {
	if m.disabled {
		return nil
	}

	m.cache.Add(string(c.Address()), c)
	return nil
}

func (m *MemStore) setCapacity(n int) {
	if n <= 0 {
		m.disabled = true
	} else {
		c, err := lru.New(n)
		if err != nil {
			panic(err)
		}

		*m = MemStore{
			cache: c,
		}
	}
}

func (s *MemStore) Close() {}
