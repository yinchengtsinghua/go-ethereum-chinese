
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

//ExplorerDockerFile是运行块资源管理器所需的DockerFile。
var explorerDockerfile = `
FROM puppeth/explorer:latest

ADD ethstats.json /ethstats.json
ADD chain.json /chain.json

RUN \
  echo '(cd ../eth-net-intelligence-api && pm2 start /ethstats.json)' >  explorer.sh && \
	echo '(cd ../etherchain-light && npm start &)'                      >> explorer.sh && \
	echo 'exec /parity/parity --chain=/chain.json --port={{.NodePort}} --tracing=on --fat-db=on --pruning=archive' >> explorer.sh

ENTRYPOINT ["/bin/sh", "explorer.sh"]
`

//explorerethstats是ethstats javascript客户机的配置文件。
var explorerEthstats = `[
  {
    "name"              : "node-app",
    "script"            : "app.js",
    "log_date_format"   : "YYYY-MM-DD HH:mm Z",
    "merge_logs"        : false,
    "watch"             : false,
    "max_restarts"      : 10,
    "exec_interpreter"  : "node",
    "exec_mode"         : "fork_mode",
    "env":
    {
      "NODE_ENV"        : "production",
      "RPC_HOST"        : "localhost",
      "RPC_PORT"        : "8545",
      "LISTENING_PORT"  : "{{.Port}}",
      "INSTANCE_NAME"   : "{{.Name}}",
      "CONTACT_DETAILS" : "",
      "WS_SERVER"       : "{{.Host}}",
      "WS_SECRET"       : "{{.Secret}}",
      "VERBOSITY"       : 2
    }
  }
]`

//explorer compose file是部署和
//维护块资源管理器。
var explorerComposefile = `
version: '2'
services:
  explorer:
    build: .
    image: {{.Network}}/explorer
    container_name: {{.Network}}_explorer_1
    ports:
      - "{{.NodePort}}:{{.NodePort}}"
      - "{{.NodePort}}:{{.NodePort}}/udp"{{if not .VHost}}
      - "{{.WebPort}}:3000"{{end}}
    volumes:
      - {{.Datadir}}:/root/.local/share/io.parity.ethereum
    environment:
      - NODE_PORT={{.NodePort}}/tcp
      - STATS={{.Ethstats}}{{if .VHost}}
      - VIRTUAL_HOST={{.VHost}}
      - VIRTUAL_PORT=3000{{end}}
    logging:
      driver: "json-file"
      options:
        max-size: "1m"
        max-file: "10"
    restart: always
`

//deployexplorer通过将新的块资源管理器容器部署到远程计算机
//ssh、docker和docker撰写。如果具有指定网络名称的实例
//已经存在，将被覆盖！
func deployExplorer(client *sshClient, network string, chainspec []byte, config *explorerInfos, nocache bool) ([]byte, error) {
//生成要上载到服务器的内容
	workdir := fmt.Sprintf("%d", rand.Int63())
	files := make(map[string][]byte)

	dockerfile := new(bytes.Buffer)
	template.Must(template.New("").Parse(explorerDockerfile)).Execute(dockerfile, map[string]interface{}{
		"NodePort": config.nodePort,
	})
	files[filepath.Join(workdir, "Dockerfile")] = dockerfile.Bytes()

	ethstats := new(bytes.Buffer)
	template.Must(template.New("").Parse(explorerEthstats)).Execute(ethstats, map[string]interface{}{
		"Port":   config.nodePort,
		"Name":   config.ethstats[:strings.Index(config.ethstats, ":")],
		"Secret": config.ethstats[strings.Index(config.ethstats, ":")+1 : strings.Index(config.ethstats, "@")],
		"Host":   config.ethstats[strings.Index(config.ethstats, "@")+1:],
	})
	files[filepath.Join(workdir, "ethstats.json")] = ethstats.Bytes()

	composefile := new(bytes.Buffer)
	template.Must(template.New("").Parse(explorerComposefile)).Execute(composefile, map[string]interface{}{
		"Datadir":  config.datadir,
		"Network":  network,
		"NodePort": config.nodePort,
		"VHost":    config.webHost,
		"WebPort":  config.webPort,
		"Ethstats": config.ethstats[:strings.Index(config.ethstats, ":")],
	})
	files[filepath.Join(workdir, "docker-compose.yaml")] = composefile.Bytes()

	files[filepath.Join(workdir, "chain.json")] = chainspec

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

//ExplorerInfo从块资源管理器状态检查返回以允许报告
//各种配置参数。
type explorerInfos struct {
	datadir  string
	ethstats string
	nodePort int
	webHost  string
	webPort  int
}

//报表将类型化结构转换为纯字符串->字符串映射，其中包含
//大多数（但不是全部）字段用于向用户报告。
func (info *explorerInfos) Report() map[string]string {
	report := map[string]string{
		"Data directory":         info.datadir,
		"Node listener port ":    strconv.Itoa(info.nodePort),
		"Ethstats username":      info.ethstats,
		"Website address ":       info.webHost,
		"Website listener port ": strconv.Itoa(info.webPort),
	}
	return report
}

//check explorer对块资源管理器服务器执行运行状况检查以验证
//
func checkExplorer(client *sshClient, network string) (*explorerInfos, error) {
//检查主机上可能的块资源管理器容器
	infos, err := inspectContainer(client, fmt.Sprintf("%s_explorer_1", network))
	if err != nil {
		return nil, err
	}
	if !infos.running {
		return nil, ErrServiceOffline
	}
//从主机或反向代理解析端口
	webPort := infos.portmap["3000/tcp"]
	if webPort == 0 {
		if proxy, _ := checkNginx(client, network); proxy != nil {
			webPort = proxy.port
		}
	}
	if webPort == 0 {
		return nil, ErrNotExposed
	}
//
	host := infos.envvars["VIRTUAL_HOST"]
	if host == "" {
		host = client.server
	}
//运行健全性检查以查看是否可以访问devp2p
	nodePort := infos.portmap[infos.envvars["NODE_PORT"]]
	if err = checkPort(client.server, nodePort); err != nil {
		log.Warn(fmt.Sprintf("Explorer devp2p port seems unreachable"), "server", client.server, "port", nodePort, "err", err)
	}
//收集并返回有用的信息
	stats := &explorerInfos{
		datadir:  infos.volumes["/root/.local/share/io.parity.ethereum"],
		nodePort: nodePort,
		webHost:  host,
		webPort:  webPort,
		ethstats: infos.envvars["STATS"],
	}
	return stats, nil
}
