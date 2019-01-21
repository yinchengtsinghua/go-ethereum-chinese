
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
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

//匹配十六进制群哈希
//托多：这很糟糕，不应该硬编码哈希值有多长
var hashMatcher = regexp.MustCompile("^([0-9A-Fa-f]{64})([0-9A-Fa-f]{64})?$")

//URI是对存储在Swarm中的内容的引用。
type URI struct {
//方案具有以下值之一：
//
//*BZZ-群清单中的条目
//*BZZ原始-原始群内容
//*BZZ不可变-群清单中某个条目的不可变URI
//（地址未解析）
//*BZZ列表-包含在Swarm清单中的所有文件的列表
//
	Scheme string

//addr是十六进制存储地址，或者是
//解析为存储地址
	Addr string

//addr存储解析的存储地址
	addr storage.Address

//路径是群清单中内容的路径
	Path string
}

func (u *URI) MarshalJSON() (out []byte, err error) {
	return []byte(`"` + u.String() + `"`), nil
}

func (u *URI) UnmarshalJSON(value []byte) error {
	uri, err := Parse(string(value))
	if err != nil {
		return err
	}
	*u = *uri
	return nil
}

//解析将rawuri解析为一个uri结构，其中rawuri应该有一个
//以下格式：
//
//＊方案>：
//*<scheme>：/<addr>
//*<scheme>：/<addr>/<path>
//＊方案>：
//*<scheme>：/<addr>
//*<scheme>：/<addr>/<path>
//
//使用方案一：bzz、bzz raw、bzz immutable、bzz list或bzz hash
func Parse(rawuri string) (*URI, error) {
	u, err := url.Parse(rawuri)
	if err != nil {
		return nil, err
	}
	uri := &URI{Scheme: u.Scheme}

//检查方案是否有效
	switch uri.Scheme {
	case "bzz", "bzz-raw", "bzz-immutable", "bzz-list", "bzz-hash", "bzz-feed":
	default:
		return nil, fmt.Errorf("unknown scheme %q", u.Scheme)
	}

//处理类似bzz://<addr>/<path>的uri，其中addr和path
//已按URL拆分。分析
	if u.Host != "" {
		uri.Addr = u.Host
		uri.Path = strings.TrimLeft(u.Path, "/")
		return uri, nil
	}

//uri类似于bzz:/<addr>/<path>so split the addr and path from
//原始路径（将是/<addr>/<path>）
	parts := strings.SplitN(strings.TrimLeft(u.Path, "/"), "/", 2)
	uri.Addr = parts[0]
	if len(parts) == 2 {
		uri.Path = parts[1]
	}
	return uri, nil
}
func (u *URI) Feed() bool {
	return u.Scheme == "bzz-feed"
}

func (u *URI) Raw() bool {
	return u.Scheme == "bzz-raw"
}

func (u *URI) Immutable() bool {
	return u.Scheme == "bzz-immutable"
}

func (u *URI) List() bool {
	return u.Scheme == "bzz-list"
}

func (u *URI) Hash() bool {
	return u.Scheme == "bzz-hash"
}

func (u *URI) String() string {
	return u.Scheme + ":/" + u.Addr + "/" + u.Path
}

func (u *URI) Address() storage.Address {
	if u.addr != nil {
		return u.addr
	}
	if hashMatcher.MatchString(u.Addr) {
		u.addr = common.Hex2Bytes(u.Addr)
		return u.addr
	}
	return nil
}
