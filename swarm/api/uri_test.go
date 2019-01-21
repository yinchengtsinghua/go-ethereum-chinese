
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

package api

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/swarm/storage"
)

func TestParseURI(t *testing.T) {
	type test struct {
		uri             string
		expectURI       *URI
		expectErr       bool
		expectRaw       bool
		expectImmutable bool
		expectList      bool
		expectHash      bool
		expectValidKey  bool
		expectAddr      storage.Address
	}
	tests := []test{
		{
			uri:       "",
			expectErr: true,
		},
		{
			uri:       "foo",
			expectErr: true,
		},
		{
			uri:       "bzz",
			expectErr: true,
		},
		{
			uri:       "bzz:",
			expectURI: &URI{Scheme: "bzz"},
		},
		{
			uri:             "bzz-immutable:",
			expectURI:       &URI{Scheme: "bzz-immutable"},
			expectImmutable: true,
		},
		{
			uri:       "bzz-raw:",
			expectURI: &URI{Scheme: "bzz-raw"},
			expectRaw: true,
		},
		{
			uri:       "bzz:/",
			expectURI: &URI{Scheme: "bzz"},
		},
		{
			uri:       "bzz:/abc123",
			expectURI: &URI{Scheme: "bzz", Addr: "abc123"},
		},
		{
			uri:       "bzz:/abc123/path/to/entry",
			expectURI: &URI{Scheme: "bzz", Addr: "abc123", Path: "path/to/entry"},
		},
		{
			uri:       "bzz-raw:/",
			expectURI: &URI{Scheme: "bzz-raw"},
			expectRaw: true,
		},
		{
			uri:       "bzz-raw:/abc123",
			expectURI: &URI{Scheme: "bzz-raw", Addr: "abc123"},
			expectRaw: true,
		},
		{
			uri:       "bzz-raw:/abc123/path/to/entry",
			expectURI: &URI{Scheme: "bzz-raw", Addr: "abc123", Path: "path/to/entry"},
			expectRaw: true,
		},
		{
uri:       "bzz://“
			expectURI: &URI{Scheme: "bzz"},
		},
		{
uri:       "bzz://ABC123“，
			expectURI: &URI{Scheme: "bzz", Addr: "abc123"},
		},
		{
uri:       "bzz://abc123/path/to/entry“，
			expectURI: &URI{Scheme: "bzz", Addr: "abc123", Path: "path/to/entry"},
		},
		{
			uri:        "bzz-hash:",
			expectURI:  &URI{Scheme: "bzz-hash"},
			expectHash: true,
		},
		{
			uri:        "bzz-hash:/",
			expectURI:  &URI{Scheme: "bzz-hash"},
			expectHash: true,
		},
		{
			uri:        "bzz-list:",
			expectURI:  &URI{Scheme: "bzz-list"},
			expectList: true,
		},
		{
			uri:        "bzz-list:/",
			expectURI:  &URI{Scheme: "bzz-list"},
			expectList: true,
		},
		{
uri: "bzz-raw://4378D19C26590F1A818ED7D6A62C3809E149B0999CAB5CE5F26233B3B423BF8C“，
			expectURI: &URI{Scheme: "bzz-raw",
				Addr: "4378d19c26590f1a818ed7d6a62c3809e149b0999cab5ce5f26233b3b423bf8c",
			},
			expectValidKey: true,
			expectRaw:      true,
			expectAddr: storage.Address{67, 120, 209, 156, 38, 89, 15, 26,
				129, 142, 215, 214, 166, 44, 56, 9,
				225, 73, 176, 153, 156, 171, 92, 229,
				242, 98, 51, 179, 180, 35, 191, 140,
			},
		},
	}
	for _, x := range tests {
		actual, err := Parse(x.uri)
		if x.expectErr {
			if err == nil {
				t.Fatalf("expected %s to error", x.uri)
			}
			continue
		}
		if err != nil {
			t.Fatalf("error parsing %s: %s", x.uri, err)
		}
		if !reflect.DeepEqual(actual, x.expectURI) {
			t.Fatalf("expected %s to return %#v, got %#v", x.uri, x.expectURI, actual)
		}
		if actual.Raw() != x.expectRaw {
			t.Fatalf("expected %s raw to be %t, got %t", x.uri, x.expectRaw, actual.Raw())
		}
		if actual.Immutable() != x.expectImmutable {
			t.Fatalf("expected %s immutable to be %t, got %t", x.uri, x.expectImmutable, actual.Immutable())
		}
		if actual.List() != x.expectList {
			t.Fatalf("expected %s list to be %t, got %t", x.uri, x.expectList, actual.List())
		}
		if actual.Hash() != x.expectHash {
			t.Fatalf("expected %s hash to be %t, got %t", x.uri, x.expectHash, actual.Hash())
		}
		if x.expectValidKey {
			if actual.Address() == nil {
				t.Fatalf("expected %s to return a valid key, got nil", x.uri)
			} else {
				if !bytes.Equal(x.expectAddr, actual.Address()) {
					t.Fatalf("expected %s to be decoded to %v", x.expectURI.Addr, x.expectAddr)
				}
			}
		}
	}
}
