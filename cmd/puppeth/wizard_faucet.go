
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
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/log"
)

//DeployFacet查询用户在部署水龙头时的各种输入，
//它执行它。
func (w *wizard) deployFaucet() {
//选择要与之交互的服务器
	server := w.selectServer()
	if server == "" {
		return
	}
	client := w.servers[server]

//从服务器检索任何活动的水龙头配置
	infos, err := checkFaucet(client, w.network)
	if err != nil {
		infos = &faucetInfos{
			node:    &nodeInfos{port: 30303, peersTotal: 25},
			port:    80,
			host:    client.server,
			amount:  1,
			minutes: 1440,
			tiers:   3,
		}
	}
	existed := err == nil

	infos.node.genesis, _ = json.MarshalIndent(w.conf.Genesis, "", "  ")
	infos.node.network = w.conf.Genesis.Config.ChainID.Int64()

//找出要监听的端口
	fmt.Println()
	fmt.Printf("Which port should the faucet listen on? (default = %d)\n", infos.port)
	infos.port = w.readDefaultInt(infos.port)

//图1部署ethstats的虚拟主机
	if infos.host, err = w.ensureVirtualHost(client, infos.port, infos.host); err != nil {
		log.Error("Failed to decide on faucet host", "err", err)
		return
	}
//检索到端口和代理设置，计算每个期间配置的资金量
	fmt.Println()
	fmt.Printf("How many Ethers to release per request? (default = %d)\n", infos.amount)
	infos.amount = w.readDefaultInt(infos.amount)

	fmt.Println()
	fmt.Printf("How many minutes to enforce between requests? (default = %d)\n", infos.minutes)
	infos.minutes = w.readDefaultInt(infos.minutes)

	fmt.Println()
	fmt.Printf("How many funding tiers to feature (x2.5 amounts, x3 timeout)? (default = %d)\n", infos.tiers)
	infos.tiers = w.readDefaultInt(infos.tiers)
	if infos.tiers == 0 {
		log.Error("At least one funding tier must be set")
		return
	}
//访问recaptcha服务需要API授权，请求它
	if infos.captchaToken != "" {
		fmt.Println()
		fmt.Println("Reuse previous reCaptcha API authorization (y/n)? (default = yes)")
		if !w.readDefaultYesNo(true) {
			infos.captchaToken, infos.captchaSecret = "", ""
		}
	}
	if infos.captchaToken == "" {
//没有以前的授权（或放弃旧的授权）
		fmt.Println()
		fmt.Println("Enable reCaptcha protection against robots (y/n)? (default = no)")
		if !w.readDefaultYesNo(false) {
			log.Warn("Users will be able to requests funds via automated scripts")
		} else {
//明确请求验证码保护，读取站点和密钥
			fmt.Println()
			fmt.Printf("What is the reCaptcha site key to authenticate human users?\n")
			infos.captchaToken = w.readString()

			fmt.Println()
			fmt.Printf("What is the reCaptcha secret key to verify authentications? (won't be echoed)\n")
			infos.captchaSecret = w.readPassword()
		}
	}
//找出用户希望存储持久数据的位置
	fmt.Println()
	if infos.node.datadir == "" {
		fmt.Printf("Where should data be stored on the remote machine?\n")
		infos.node.datadir = w.readString()
	} else {
		fmt.Printf("Where should data be stored on the remote machine? (default = %s)\n", infos.node.datadir)
		infos.node.datadir = w.readDefaultString(infos.node.datadir)
	}
//找出要监听的端口
	fmt.Println()
	fmt.Printf("Which TCP/UDP port should the light client listen on? (default = %d)\n", infos.node.port)
	infos.node.port = w.readDefaultInt(infos.node.port)

//
	fmt.Println()
	if infos.node.ethstats == "" {
		fmt.Printf("What should the node be called on the stats page?\n")
		infos.node.ethstats = w.readString() + ":" + w.conf.ethstats
	} else {
		fmt.Printf("What should the node be called on the stats page? (default = %s)\n", infos.node.ethstats)
		infos.node.ethstats = w.readDefaultString(infos.node.ethstats) + ":" + w.conf.ethstats
	}
//加载释放资金所需的凭证
	if infos.node.keyJSON != "" {
		if key, err := keystore.DecryptKey([]byte(infos.node.keyJSON), infos.node.keyPass); err != nil {
			infos.node.keyJSON, infos.node.keyPass = "", ""
		} else {
			fmt.Println()
			fmt.Printf("Reuse previous (%s) funding account (y/n)? (default = yes)\n", key.Address.Hex())
			if !w.readDefaultYesNo(true) {
				infos.node.keyJSON, infos.node.keyPass = "", ""
			}
		}
	}
	for i := 0; i < 3 && infos.node.keyJSON == ""; i++ {
		fmt.Println()
		fmt.Println("Please paste the faucet's funding account key JSON:")
		infos.node.keyJSON = w.readJSON()

		fmt.Println()
		fmt.Println("What's the unlock password for the account? (won't be echoed)")
		infos.node.keyPass = w.readPassword()

		if _, err := keystore.DecryptKey([]byte(infos.node.keyJSON), infos.node.keyPass); err != nil {
			log.Error("Failed to decrypt key with given passphrase")
			infos.node.keyJSON = ""
			infos.node.keyPass = ""
		}
	}
//
	noauth := "n"
	if infos.noauth {
		noauth = "y"
	}
	fmt.Println()
	fmt.Printf("Permit non-authenticated funding requests (y/n)? (default = %v)\n", infos.noauth)
	infos.noauth = w.readDefaultString(noauth) != "n"

//尝试在主机上部署水龙头服务器
	nocache := false
	if existed {
		fmt.Println()
		fmt.Printf("Should the faucet be built from scratch (y/n)? (default = no)\n")
		nocache = w.readDefaultYesNo(false)
	}
	if out, err := deployFaucet(client, w.network, w.conf.bootnodes, infos, nocache); err != nil {
		log.Error("Failed to deploy faucet container", "err", err)
		if len(out) > 0 {
			fmt.Printf("%s\n", out)
		}
		return
	}
//一切正常，运行网络扫描以获取任何更改
	w.networkStats()
}
