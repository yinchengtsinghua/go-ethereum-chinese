
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
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/log"
)

//
func makeWizard(network string) *wizard {
	return &wizard{
		network: network,
		conf: config{
			Servers: make(map[string][]byte),
		},
		servers:  make(map[string]*sshClient),
		services: make(map[string][]string),
		in:       bufio.NewReader(os.Stdin),
	}
}

//
//
func (w *wizard) run() {
	fmt.Println("+-----------------------------------------------------------+")
	fmt.Println("| Welcome to puppeth, your Ethereum private network manager |")
	fmt.Println("|                                                           |")
	fmt.Println("| This tool lets you create a new Ethereum network down to  |")
	fmt.Println("| the genesis block, bootnodes, miners and ethstats servers |")
	fmt.Println("| without the hassle that it would normally entail.         |")
	fmt.Println("|                                                           |")
	fmt.Println("| Puppeth uses SSH to dial in to remote servers, and builds |")
	fmt.Println("| its network components out of Docker containers using the |")
	fmt.Println("| docker-compose toolset.                                   |")
	fmt.Println("+-----------------------------------------------------------+")
	fmt.Println()

//确保我们有一个好的网络名称来使用fmt.println（）。
//Docker接受图像名称中的连字符，但不喜欢容器名称中的连字符
	if w.network == "" {
		fmt.Println("Please specify a network name to administer (no spaces, hyphens or capital letters please)")
		for {
			w.network = w.readString()
			if !strings.Contains(w.network, " ") && !strings.Contains(w.network, "-") && strings.ToLower(w.network) == w.network {
				fmt.Printf("\nSweet, you can set this via --network=%s next time!\n\n", w.network)
				break
			}
			log.Error("I also like to live dangerously, still no spaces, hyphens or capital letters")
		}
	}
	log.Info("Administering Ethereum network", "name", w.network)

//加载初始配置并连接到所有活动服务器
	w.conf.path = filepath.Join(os.Getenv("HOME"), ".puppeth", w.network)

	blob, err := ioutil.ReadFile(w.conf.path)
	if err != nil {
		log.Warn("No previous configurations found", "path", w.conf.path)
	} else if err := json.Unmarshal(blob, &w.conf); err != nil {
		log.Crit("Previous configuration corrupted", "path", w.conf.path, "err", err)
	} else {
//同时拨打所有以前已知的服务器
		var pend sync.WaitGroup
		for server, pubkey := range w.conf.Servers {
			pend.Add(1)

			go func(server string, pubkey []byte) {
				defer pend.Done()

				log.Info("Dialing previously configured server", "server", server)
				client, err := dial(server, pubkey)
				if err != nil {
					log.Error("Previous server unreachable", "server", server, "err", err)
				}
				w.lock.Lock()
				w.servers[server] = client
				w.lock.Unlock()
			}(server, pubkey)
		}
		pend.Wait()
		w.networkStats()
	}
//基本完成了，无限循环关于该做什么
	for {
		fmt.Println()
		fmt.Println("What would you like to do? (default = stats)")
		fmt.Println(" 1. Show network stats")
		if w.conf.Genesis == nil {
			fmt.Println(" 2. Configure new genesis")
		} else {
			fmt.Println(" 2. Manage existing genesis")
		}
		if len(w.servers) == 0 {
			fmt.Println(" 3. Track new remote server")
		} else {
			fmt.Println(" 3. Manage tracked machines")
		}
		if len(w.services) == 0 {
			fmt.Println(" 4. Deploy network components")
		} else {
			fmt.Println(" 4. Manage network components")
		}

		choice := w.read()
		switch {
		case choice == "" || choice == "1":
			w.networkStats()

		case choice == "2":
			if w.conf.Genesis == nil {
				fmt.Println()
				fmt.Println("What would you like to do? (default = create)")
				fmt.Println(" 1. Create new genesis from scratch")
				fmt.Println(" 2. Import already existing genesis")

				choice := w.read()
				switch {
				case choice == "" || choice == "1":
					w.makeGenesis()
				case choice == "2":
					w.importGenesis()
				default:
					log.Error("That's not something I can do")
				}
			} else {
				w.manageGenesis()
			}
		case choice == "3":
			if len(w.servers) == 0 {
				if w.makeServer() != "" {
					w.networkStats()
				}
			} else {
				w.manageServers()
			}
		case choice == "4":
			if len(w.services) == 0 {
				w.deployComponent()
			} else {
				w.manageComponents()
			}
		default:
			log.Error("That's not something I can do")
		}
	}
}
