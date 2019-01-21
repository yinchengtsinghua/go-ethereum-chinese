
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

	"github.com/ethereum/go-ethereum/log"
)

//
//代理。
var nginxDockerfile = `FROM jwilder/nginx-proxy`

//
//
//在单个主机上运行的服务。
var nginxComposefile = `
version: '2'
services:
  nginx:
    build: .
    image: {{.Network}}/nginx
    container_name: {{.Network}}_nginx_1
    ports:
      - "{{.Port}}:80"
    volumes:
      - /var/run/docker.sock:/tmp/docker.sock:ro
    logging:
      driver: "json-file"
      options:
        max-size: "1m"
        max-file: "10"
    restart: always
`

//
//在单个主机上运行的HTTP服务。如果具有指定的
//网络名已经存在，将被覆盖！
func deployNginx(client *sshClient, network string, port int, nocache bool) ([]byte, error) {
	log.Info("Deploying nginx reverse-proxy", "server", client.server, "port", port)

//生成要上载到服务器的内容
	workdir := fmt.Sprintf("%d", rand.Int63())
	files := make(map[string][]byte)

	dockerfile := new(bytes.Buffer)
	template.Must(template.New("").Parse(nginxDockerfile)).Execute(dockerfile, nil)
	files[filepath.Join(workdir, "Dockerfile")] = dockerfile.Bytes()

	composefile := new(bytes.Buffer)
	template.Must(template.New("").Parse(nginxComposefile)).Execute(composefile, map[string]interface{}{
		"Network": network,
		"Port":    port,
	})
	files[filepath.Join(workdir, "docker-compose.yaml")] = composefile.Bytes()

//将部署文件上载到远程服务器（然后清理）
	if out, err := client.Upload(files); err != nil {
		return out, err
	}
	defer client.Run("rm -rf " + workdir)

//
	if nocache {
		return nil, client.Stream(fmt.Sprintf("cd %s && docker-compose -p %s build --pull --no-cache && docker-compose -p %s up -d --force-recreate --timeout 60", workdir, network, network))
	}
	return nil, client.Stream(fmt.Sprintf("cd %s && docker-compose -p %s up -d --build --force-recreate --timeout 60", workdir, network))
}

//nginxinfos从nginx反向代理状态检查返回以允许
//
type nginxInfos struct {
	port int
}

//报表将类型化结构转换为纯字符串->字符串映射，其中包含
//大多数（但不是全部）字段用于向用户报告。
func (info *nginxInfos) Report() map[string]string {
	return map[string]string{
		"Shared listener port": strconv.Itoa(info.port),
	}
}

//check nginx对nginx反向代理进行健康检查，以验证
//它正在运行，如果是，收集有关它的有用信息。
func checkNginx(client *sshClient, network string) (*nginxInfos, error) {
//
	infos, err := inspectContainer(client, fmt.Sprintf("%s_nginx_1", network))
	if err != nil {
		return nil, err
	}
	if !infos.running {
		return nil, ErrServiceOffline
	}
//容器可用，组装并返回有用的信息
	return &nginxInfos{
		port: infos.portmap["80/tcp"],
	}, nil
}
