
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
//此文件是Go以太坊的一部分。
//
//Go以太坊是免费软件：您可以重新发布和/或修改它
//根据GNU通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊的分布希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU通用公共许可证了解更多详细信息。
//
//你应该已经收到一份GNU通用公共许可证的副本
//一起去以太坊吧。如果没有，请参见<http://www.gnu.org/licenses/>。

package main

import (
	"fmt"

	"github.com/ethereum/go-ethereum/log"
)

//确保revirtualhost检查是否在指定的
//
//
//
//
//
func (w *wizard) ensureVirtualHost(client *sshClient, port int, def string) (string, error) {
	proxy, _ := checkNginx(client, w.network)
	if proxy != nil {
//反向代理正在运行，如果端口匹配，我们需要一个虚拟主机
		if proxy.port == port {
			fmt.Println()
			fmt.Printf("Shared port, which domain to assign? (default = %s)\n", def)
			return w.readDefaultString(def), nil
		}
	}
//
	fmt.Println()
	fmt.Println("Allow sharing the port with other services (y/n)? (default = yes)")
	if w.readDefaultYesNo(true) {
		nocache := false
		if proxy != nil {
			fmt.Println()
			fmt.Printf("Should the reverse-proxy be rebuilt from scratch (y/n)? (default = no)\n")
			nocache = w.readDefaultYesNo(false)
		}
		if out, err := deployNginx(client, w.network, port, nocache); err != nil {
			log.Error("Failed to deploy reverse-proxy", "err", err)
			if len(out) > 0 {
				fmt.Printf("%s\n", out)
			}
			return "", err
		}
//已部署反向代理，请再次请求虚拟主机
		fmt.Println()
		fmt.Printf("Proxy deployed, which domain to assign? (default = %s)\n", def)
		return w.readDefaultString(def), nil
	}
//
	return "", nil
}
