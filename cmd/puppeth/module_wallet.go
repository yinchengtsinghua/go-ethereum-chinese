
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
	"bytes"
	"fmt"
	"html/template"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/log"
)

//
var walletDockerfile = `
FROM puppeth/wallet:latest

ADD genesis.json /genesis.json

RUN \
  echo 'node server.js &'                     > wallet.sh && \
	echo 'geth --cache 512 init /genesis.json' >> wallet.sh && \
	echo $'exec geth --networkid {{.NetworkID}} --port {{.NodePort}} --bootnodes {{.Bootnodes}} --ethstats \'{{.Ethstats}}\' --cache=512 --rpc --rpcaddr=0.0.0.0 --rpccorsdomain "*" --rpcvhosts "*"' >> wallet.sh

RUN \
	sed -i 's/PuppethNetworkID/{{.NetworkID}}/g' dist/js/etherwallet-master.js && \
	sed -i 's/PuppethNetwork/{{.Network}}/g'     dist/js/etherwallet-master.js && \
	sed -i 's/PuppethDenom/{{.Denom}}/g'         dist/js/etherwallet-master.js && \
	sed -i 's/PuppethHost/{{.Host}}/g'           dist/js/etherwallet-master.js && \
	sed -i 's/PuppethRPCPort/{{.RPCPort}}/g'     dist/js/etherwallet-master.js

ENTRYPOINT ["/bin/sh", "wallet.sh"]
`

//walletcomposefile是部署和
//
var walletComposefile = `
version: '2'
services:
  wallet:
    build: .
    image: {{.Network}}/wallet
    container_name: {{.Network}}_wallet_1
    ports:
      - "{{.NodePort}}:{{.NodePort}}"
      - "{{.NodePort}}:{{.NodePort}}/udp"
      - "{{.RPCPort}}:8545"{{if not .VHost}}
      - "{{.WebPort}}:80"{{end}}
    volumes:
      - {{.Datadir}}:/root/.ethereum
    environment:
      - NODE_PORT={{.NodePort}}/tcp
      - STATS={{.Ethstats}}{{if .VHost}}
      - VIRTUAL_HOST={{.VHost}}
      - VIRTUAL_PORT=80{{end}}
    logging:
      driver: "json-file"
      options:
        max-size: "1m"
        max-file: "10"
    restart: always
`

//
//Docker和Docker组合。如果具有指定网络名称的实例
//已经存在，将被覆盖！
func deployWallet(client *sshClient, network string, bootnodes []string, config *walletInfos, nocache bool) ([]byte, error) {
//生成要上载到服务器的内容
	workdir := fmt.Sprintf("%d", rand.Int63())
	files := make(map[string][]byte)

	dockerfile := new(bytes.Buffer)
	template.Must(template.New("").Parse(walletDockerfile)).Execute(dockerfile, map[string]interface{}{
		"Network":   strings.ToTitle(network),
		"Denom":     strings.ToUpper(network),
		"NetworkID": config.network,
		"NodePort":  config.nodePort,
		"RPCPort":   config.rpcPort,
		"Bootnodes": strings.Join(bootnodes, ","),
		"Ethstats":  config.ethstats,
		"Host":      client.address,
	})
	files[filepath.Join(workdir, "Dockerfile")] = dockerfile.Bytes()

	composefile := new(bytes.Buffer)
	template.Must(template.New("").Parse(walletComposefile)).Execute(composefile, map[string]interface{}{
		"Datadir":  config.datadir,
		"Network":  network,
		"NodePort": config.nodePort,
		"RPCPort":  config.rpcPort,
		"VHost":    config.webHost,
		"WebPort":  config.webPort,
		"Ethstats": config.ethstats[:strings.Index(config.ethstats, ":")],
	})
	files[filepath.Join(workdir, "docker-compose.yaml")] = composefile.Bytes()

	files[filepath.Join(workdir, "genesis.json")] = config.genesis

//将部署文件上载到远程服务器（然后清理）
	if out, err := client.Upload(files); err != nil {
		return out, err
	}
	defer client.Run("rm -rf " + workdir)

//构建和部署引导或密封节点服务
	if nocache {
		return nil, client.Stream(fmt.Sprintf("cd %s && docker-compose -p %s build --pull --no-cache && docker-compose -p %s up -d --force-recreate --timeout 60", workdir, network, network))
	}
	return nil, client.Stream(fmt.Sprintf("cd %s && docker-compose -p %s up -d --build --force-recreate --timeout 60", workdir, network))
}

//
//各种配置参数。
type walletInfos struct {
	genesis  []byte
	network  int64
	datadir  string
	ethstats string
	nodePort int
	rpcPort  int
	webHost  string
	webPort  int
}

//报表将类型化结构转换为纯字符串->字符串映射，其中包含
//大多数（但不是全部）字段用于向用户报告。
func (info *walletInfos) Report() map[string]string {
	report := map[string]string{
		"Data directory":         info.datadir,
		"Ethstats username":      info.ethstats,
		"Node listener port ":    strconv.Itoa(info.nodePort),
		"RPC listener port ":     strconv.Itoa(info.rpcPort),
		"Website address ":       info.webHost,
		"Website listener port ": strconv.Itoa(info.webPort),
	}
	return report
}

//check wallet对web wallet服务器进行健康检查，以验证
//
func checkWallet(client *sshClient, network string) (*walletInfos, error) {
//
	infos, err := inspectContainer(client, fmt.Sprintf("%s_wallet_1", network))
	if err != nil {
		return nil, err
	}
	if !infos.running {
		return nil, ErrServiceOffline
	}
//从主机或反向代理解析端口
	webPort := infos.portmap["80/tcp"]
	if webPort == 0 {
		if proxy, _ := checkNginx(client, network); proxy != nil {
			webPort = proxy.port
		}
	}
	if webPort == 0 {
		return nil, ErrNotExposed
	}
//从反向代理和配置值解析主机
	host := infos.envvars["VIRTUAL_HOST"]
	if host == "" {
		host = client.server
	}
//
	nodePort := infos.portmap[infos.envvars["NODE_PORT"]]
	if err = checkPort(client.server, nodePort); err != nil {
		log.Warn(fmt.Sprintf("Wallet devp2p port seems unreachable"), "server", client.server, "port", nodePort, "err", err)
	}
	rpcPort := infos.portmap["8545/tcp"]
	if err = checkPort(client.server, rpcPort); err != nil {
		log.Warn(fmt.Sprintf("Wallet RPC port seems unreachable"), "server", client.server, "port", rpcPort, "err", err)
	}
//收集并返回有用的信息
	stats := &walletInfos{
		datadir:  infos.volumes["/root/.ethereum"],
		nodePort: nodePort,
		rpcPort:  rpcPort,
		webHost:  host,
		webPort:  webPort,
		ethstats: infos.envvars["STATS"],
	}
	return stats, nil
}
