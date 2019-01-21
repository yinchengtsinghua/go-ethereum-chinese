
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
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

//
//
var faucetDockerfile = `
FROM ethereum/client-go:alltools-latest

ADD genesis.json /genesis.json
ADD account.json /account.json
ADD account.pass /account.pass

EXPOSE 8080 30303 30303/udp

ENTRYPOINT [ \
	"faucet", "--genesis", "/genesis.json", "--network", "{{.NetworkID}}", "--bootnodes", "{{.Bootnodes}}", "--ethstats", "{{.Ethstats}}", "--ethport", "{{.EthPort}}",     \
	"--faucet.name", "{{.FaucetName}}", "--faucet.amount", "{{.FaucetAmount}}", "--faucet.minutes", "{{.FaucetMinutes}}", "--faucet.tiers", "{{.FaucetTiers}}",             \
	"--account.json", "/account.json", "--account.pass", "/account.pass"                                                                                                    \
	{{if .CaptchaToken}}, "--captcha.token", "{{.CaptchaToken}}", "--captcha.secret", "{{.CaptchaSecret}}"{{end}}{{if .NoAuth}}, "--noauth"{{end}}                          \
]`

//水龙头组合文件是部署和维护所需的docker-compose.yml文件。
//加密水龙头。
var faucetComposefile = `
version: '2'
services:
  faucet:
    build: .
    image: {{.Network}}/faucet
    container_name: {{.Network}}_faucet_1
    ports:
      - "{{.EthPort}}:{{.EthPort}}"
      - "{{.EthPort}}:{{.EthPort}}/udp"{{if not .VHost}}
      - "{{.ApiPort}}:8080"{{end}}
    volumes:
      - {{.Datadir}}:/root/.faucet
    environment:
      - ETH_PORT={{.EthPort}}
      - ETH_NAME={{.EthName}}
      - FAUCET_AMOUNT={{.FaucetAmount}}
      - FAUCET_MINUTES={{.FaucetMinutes}}
      - FAUCET_TIERS={{.FaucetTiers}}
      - CAPTCHA_TOKEN={{.CaptchaToken}}
      - CAPTCHA_SECRET={{.CaptchaSecret}}
      - NO_AUTH={{.NoAuth}}{{if .VHost}}
      - VIRTUAL_HOST={{.VHost}}
      - VIRTUAL_PORT=8080{{end}}
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
func deployFaucet(client *sshClient, network string, bootnodes []string, config *faucetInfos, nocache bool) ([]byte, error) {
//生成要上载到服务器的内容
	workdir := fmt.Sprintf("%d", rand.Int63())
	files := make(map[string][]byte)

	dockerfile := new(bytes.Buffer)
	template.Must(template.New("").Parse(faucetDockerfile)).Execute(dockerfile, map[string]interface{}{
		"NetworkID":     config.node.network,
		"Bootnodes":     strings.Join(bootnodes, ","),
		"Ethstats":      config.node.ethstats,
		"EthPort":       config.node.port,
		"CaptchaToken":  config.captchaToken,
		"CaptchaSecret": config.captchaSecret,
		"FaucetName":    strings.Title(network),
		"FaucetAmount":  config.amount,
		"FaucetMinutes": config.minutes,
		"FaucetTiers":   config.tiers,
		"NoAuth":        config.noauth,
	})
	files[filepath.Join(workdir, "Dockerfile")] = dockerfile.Bytes()

	composefile := new(bytes.Buffer)
	template.Must(template.New("").Parse(faucetComposefile)).Execute(composefile, map[string]interface{}{
		"Network":       network,
		"Datadir":       config.node.datadir,
		"VHost":         config.host,
		"ApiPort":       config.port,
		"EthPort":       config.node.port,
		"EthName":       config.node.ethstats[:strings.Index(config.node.ethstats, ":")],
		"CaptchaToken":  config.captchaToken,
		"CaptchaSecret": config.captchaSecret,
		"FaucetAmount":  config.amount,
		"FaucetMinutes": config.minutes,
		"FaucetTiers":   config.tiers,
		"NoAuth":        config.noauth,
	})
	files[filepath.Join(workdir, "docker-compose.yaml")] = composefile.Bytes()

	files[filepath.Join(workdir, "genesis.json")] = config.node.genesis
	files[filepath.Join(workdir, "account.json")] = []byte(config.node.keyJSON)
	files[filepath.Join(workdir, "account.pass")] = []byte(config.node.keyPass)

//将部署文件上载到远程服务器（然后清理）
	if out, err := client.Upload(files); err != nil {
		return out, err
	}
	defer client.Run("rm -rf " + workdir)

//建立和部署水龙头服务
	if nocache {
		return nil, client.Stream(fmt.Sprintf("cd %s && docker-compose -p %s build --pull --no-cache && docker-compose -p %s up -d --force-recreate --timeout 60", workdir, network, network))
	}
	return nil, client.Stream(fmt.Sprintf("cd %s && docker-compose -p %s up -d --build --force-recreate --timeout 60", workdir, network))
}

//水龙头信息从水龙头状态检查返回，以允许报告各种
//配置参数。
type faucetInfos struct {
	node          *nodeInfos
	host          string
	port          int
	amount        int
	minutes       int
	tiers         int
	noauth        bool
	captchaToken  string
	captchaSecret string
}

//报表将类型化结构转换为纯字符串->字符串映射，其中包含
//大多数（但不是全部）字段用于向用户报告。
func (info *faucetInfos) Report() map[string]string {
	report := map[string]string{
		"Website address":              info.host,
		"Website listener port":        strconv.Itoa(info.port),
		"Ethereum listener port":       strconv.Itoa(info.node.port),
		"Funding amount (base tier)":   fmt.Sprintf("%d Ethers", info.amount),
		"Funding cooldown (base tier)": fmt.Sprintf("%d mins", info.minutes),
		"Funding tiers":                strconv.Itoa(info.tiers),
		"Captha protection":            fmt.Sprintf("%v", info.captchaToken != ""),
		"Ethstats username":            info.node.ethstats,
	}
	if info.noauth {
		report["Debug mode (no auth)"] = "enabled"
	}
	if info.node.keyJSON != "" {
		var key struct {
			Address string `json:"address"`
		}
		if err := json.Unmarshal([]byte(info.node.keyJSON), &key); err == nil {
			report["Funding account"] = common.HexToAddress(key.Address).Hex()
		} else {
			log.Error("Failed to retrieve signer address", "err", err)
		}
	}
	return report
}

//检查水龙头是否对水龙头服务器进行健康检查以验证
//它正在运行，如果是，收集有关它的有用信息。
func checkFaucet(client *sshClient, network string) (*faucetInfos, error) {
//
	infos, err := inspectContainer(client, fmt.Sprintf("%s_faucet_1", network))
	if err != nil {
		return nil, err
	}
	if !infos.running {
		return nil, ErrServiceOffline
	}
//从主机或反向代理解析端口
	port := infos.portmap["8080/tcp"]
	if port == 0 {
		if proxy, _ := checkNginx(client, network); proxy != nil {
			port = proxy.port
		}
	}
	if port == 0 {
		return nil, ErrNotExposed
	}
//从反向代理和配置值解析主机
	host := infos.envvars["VIRTUAL_HOST"]
	if host == "" {
		host = client.server
	}
	amount, _ := strconv.Atoi(infos.envvars["FAUCET_AMOUNT"])
	minutes, _ := strconv.Atoi(infos.envvars["FAUCET_MINUTES"])
	tiers, _ := strconv.Atoi(infos.envvars["FAUCET_TIERS"])

//检索资金帐户信息
	var out []byte
	keyJSON, keyPass := "", ""
	if out, err = client.Run(fmt.Sprintf("docker exec %s_faucet_1 cat /account.json", network)); err == nil {
		keyJSON = string(bytes.TrimSpace(out))
	}
	if out, err = client.Run(fmt.Sprintf("docker exec %s_faucet_1 cat /account.pass", network)); err == nil {
		keyPass = string(bytes.TrimSpace(out))
	}
//运行健全检查以查看端口是否可访问
	if err = checkPort(host, port); err != nil {
		log.Warn("Faucet service seems unreachable", "server", host, "port", port, "err", err)
	}
//容器可用，组装并返回有用的信息
	return &faucetInfos{
		node: &nodeInfos{
			datadir:  infos.volumes["/root/.faucet"],
			port:     infos.portmap[infos.envvars["ETH_PORT"]+"/tcp"],
			ethstats: infos.envvars["ETH_NAME"],
			keyJSON:  keyJSON,
			keyPass:  keyPass,
		},
		host:          host,
		port:          port,
		amount:        amount,
		minutes:       minutes,
		tiers:         tiers,
		captchaToken:  infos.envvars["CAPTCHA_TOKEN"],
		captchaSecret: infos.envvars["CAPTCHA_SECRET"],
		noauth:        infos.envvars["NO_AUTH"] == "true",
	}, nil
}
