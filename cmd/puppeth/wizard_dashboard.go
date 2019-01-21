
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

//
//
func (w *wizard) deployDashboard() {
//
	server := w.selectServer()
	if server == "" {
		return
	}
	client := w.servers[server]

//
	infos, err := checkDashboard(client, w.network)
	if err != nil {
		infos = &dashboardInfos{
			port: 80,
			host: client.server,
		}
	}
	existed := err == nil

//找出要监听的端口
	fmt.Println()
	fmt.Printf("Which port should the dashboard listen on? (default = %d)\n", infos.port)
	infos.port = w.readDefaultInt(infos.port)

//
	infos.host, err = w.ensureVirtualHost(client, infos.port, infos.host)
	if err != nil {
		log.Error("Failed to decide on dashboard host", "err", err)
		return
	}
//检索到端口和代理设置，确定哪些服务可用
	available := make(map[string][]string)
	for server, services := range w.services {
		for _, service := range services {
			available[service] = append(available[service], server)
		}
	}
	for _, service := range []string{"ethstats", "explorer", "wallet", "faucet"} {
//收集此类型的所有本地宿主页
		var pages []string
		for _, server := range available[service] {
			client := w.servers[server]
			if client == nil {
				continue
			}
//如果机器上有运行的服务，检索它的端口号
			var port int
			switch service {
			case "ethstats":
				if infos, err := checkEthstats(client, w.network); err == nil {
					port = infos.port
				}
			case "explorer":
				if infos, err := checkExplorer(client, w.network); err == nil {
					port = infos.webPort
				}
			case "wallet":
				if infos, err := checkWallet(client, w.network); err == nil {
					port = infos.webPort
				}
			case "faucet":
				if infos, err := checkFaucet(client, w.network); err == nil {
					port = infos.port
				}
			}
			if page, err := resolve(client, w.network, service, port); err == nil && page != "" {
				pages = append(pages, page)
			}
		}
//
		defLabel, defChoice := "don't list", len(pages)+2
		if len(pages) > 0 {
			defLabel, defChoice = pages[0], 1
		}
		fmt.Println()
		fmt.Printf("Which %s service to list? (default = %s)\n", service, defLabel)
		for i, page := range pages {
			fmt.Printf(" %d. %s\n", i+1, page)
		}
		fmt.Printf(" %d. List external %s service\n", len(pages)+1, service)
		fmt.Printf(" %d. Don't list any %s service\n", len(pages)+2, service)

		choice := w.readDefaultInt(defChoice)
		if choice < 0 || choice > len(pages)+2 {
			log.Error("Invalid listing choice, aborting")
			return
		}
		var page string
		switch {
		case choice <= len(pages):
			page = pages[choice-1]
		case choice == len(pages)+1:
			fmt.Println()
			fmt.Printf("Which address is the external %s service at?\n", service)
			page = w.readString()
		default:
//没有此的服务宿主
		}
//保存用户选择
		switch service {
		case "ethstats":
			infos.ethstats = page
		case "explorer":
			infos.explorer = page
		case "wallet":
			infos.wallet = page
		case "faucet":
			infos.faucet = page
		}
	}
//如果我们在运行ethstats，询问是否公开这个秘密
	if w.conf.ethstats != "" {
		fmt.Println()
		fmt.Println("Include ethstats secret on dashboard (y/n)? (default = yes)")
		infos.trusted = w.readDefaultYesNo(true)
	}
//
	nocache := false
	if existed {
		fmt.Println()
		fmt.Printf("Should the dashboard be built from scratch (y/n)? (default = no)\n")
		nocache = w.readDefaultYesNo(false)
	}
	if out, err := deployDashboard(client, w.network, &w.conf, infos, nocache); err != nil {
		log.Error("Failed to deploy dashboard container", "err", err)
		if len(out) > 0 {
			fmt.Printf("%s\n", out)
		}
		return
	}
//
	w.networkStats()
}
