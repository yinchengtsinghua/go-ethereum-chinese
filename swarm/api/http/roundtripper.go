
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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

package http

import (
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/swarm/log"
)

/*
注册BZZ URL方案的HTTP往返器
请参阅https://github.com/ethereum/go-ethereum/issues/2040
用途：

进口（
 “github.com/ethereum/go-ethereum/common/httpclient”
 “github.com/ethereum/go-ethereum/swarm/api/http”
）
客户端：=httpclient.new（）
//对于本地运行的（私有）Swarm代理
client.registerscheme（“bzz”，&http.roundtripper port:port）
client.registerscheme（“bzz不可变”，&http.roundtripper port:port）
client.registerscheme（“bzz raw”，&http.roundtripper port:port）

您给往返者的端口是Swarm代理正在监听的端口。
如果主机为空，则假定为localhost。

使用公共网关，上面的几条线为您提供了最精简的
BZZ方案感知只读HTTP客户端。你真的只需要这个
如果你需要本地群访问BZZ地址。
**/


type RoundTripper struct {
	Host string
	Port string
}

func (self *RoundTripper) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	host := self.Host
	if len(host) == 0 {
		host = "localhost"
	}
url := fmt.Sprintf("http://%s:%s/%s/%s/%s”，主机，自身端口，请求协议，请求URL.host，请求URL.path）
	log.Info(fmt.Sprintf("roundtripper: proxying request '%s' to '%s'", req.RequestURI, url))
	reqProxy, err := http.NewRequest(req.Method, url, req.Body)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(reqProxy)
}
