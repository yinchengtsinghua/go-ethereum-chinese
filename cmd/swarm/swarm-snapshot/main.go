
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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
	"os"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/log"
	cli "gopkg.in/urfave/cli.v1"
)

var gitCommit string //git sha1提交发布的哈希（通过链接器标志设置）

//“创建”命令的默认值--节点标志
const defaultNodes = 10

func main() {
	err := newApp().Run(os.Args)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}

//newapp构建了一个新的swarm快照实用程序实例。
//方法run在主函数和测试中对其进行调用。
func newApp() (app *cli.App) {
	app = utils.NewApp(gitCommit, "Swarm Snapshot Utility")

	app.Name = "swarm-snapshot"
	app.Usage = ""

//应用程序标志（用于所有命令）
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "verbosity",
			Value: 1,
			Usage: "verbosity level",
		},
	}

	app.Commands = []cli.Command{
		{
			Name:    "create",
			Aliases: []string{"c"},
			Usage:   "create a swarm snapshot",
			Action:  create,
//仅用于“创建”命令的标志。
//允许在
//命令参数。
			Flags: append(app.Flags,
				cli.IntFlag{
					Name:  "nodes",
					Value: defaultNodes,
					Usage: "number of nodes",
				},
				cli.StringFlag{
					Name:  "services",
					Value: "bzz",
					Usage: "comma separated list of services to boot the nodes with",
				},
			),
		},
	}

	return app
}
