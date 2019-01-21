
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

package accounts

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

//URL表示钱包或帐户的规范标识URL。
//
//这是url.url的简化版本，与重要的限制（这
//在这里被认为是特性）它只包含值可复制组件，
//而且它不做任何特殊字符的URL编码/解码。
//
//前者是很重要的，它允许在不离开Live的情况下复制帐户。
//引用原始版本，而后者对于确保
//一个单一的规范形式与RFC3986规范中允许的许多形式相反。
//
//因此，这些URL不应在以太坊范围之外使用。
//钱包或账户。
type URL struct {
Scheme string //用于标识可用帐户后端的协议方案
Path   string //用于标识唯一实体的后端路径
}

//ParseURL将用户提供的URL转换为特定于帐户的结构。
func parseURL(url string) (URL, error) {
parts := strings.Split(url, "://“”
	if len(parts) != 2 || parts[0] == "" {
		return URL{}, errors.New("protocol scheme missing")
	}
	return URL{
		Scheme: parts[0],
		Path:   parts[1],
	}, nil
}

//字符串实现字符串接口。
func (u URL) String() string {
	if u.Scheme != "" {
return fmt.Sprintf("%s://%s“，美国方案，美国路径）
	}
	return u.Path
}

//TerminalString实现Log.TerminalStringer接口。
func (u URL) TerminalString() string {
	url := u.String()
	if len(url) > 32 {
		return url[:31] + "…"
	}
	return url
}

//marshaljson实现json.marshaller接口。
func (u URL) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

//unmashaljson解析URL。
func (u *URL) UnmarshalJSON(input []byte) error {
	var textURL string
	err := json.Unmarshal(input, &textURL)
	if err != nil {
		return err
	}
	url, err := parseURL(textURL)
	if err != nil {
		return err
	}
	u.Scheme = url.Scheme
	u.Path = url.Path
	return nil
}

//CMP比较X和Y并返回：
//
//-如果x＜1
//0如果x=＝y
//如果x＞y，则为1
//
func (u URL) Cmp(url URL) int {
	if u.Scheme == url.Scheme {
		return strings.Compare(u.Path, url.Path)
	}
	return strings.Compare(u.Scheme, url.Scheme)
}
