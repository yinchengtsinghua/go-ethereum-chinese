
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
	"os"
	"strings"
	"text/tabwriter"

	"github.com/ethereum/go-ethereum/cmd/utils"
	swarm "github.com/ethereum/go-ethereum/swarm/api/client"
	"gopkg.in/urfave/cli.v1"
)

var listCommand = cli.Command{
	Action:             list,
	CustomHelpTemplate: helpTemplate,
	Name:               "ls",
	Usage:              "list files and directories contained in a manifest",
	ArgsUsage:          "<manifest> [<prefix>]",
	Description:        "Lists files and directories contained in a manifest",
}

func list(ctx *cli.Context) {
	args := ctx.Args()

	if len(args) < 1 {
		utils.Fatalf("Please supply a manifest reference as the first argument")
	} else if len(args) > 2 {
		utils.Fatalf("Too many arguments - usage 'swarm ls manifest [prefix]'")
	}
	manifest := args[0]

	var prefix string
	if len(args) == 2 {
		prefix = args[1]
	}

	bzzapi := strings.TrimRight(ctx.GlobalString(SwarmApiFlag.Name), "/")
	client := swarm.NewClient(bzzapi)
	list, err := client.List(manifest, prefix, "")
	if err != nil {
		utils.Fatalf("Failed to generate file and directory list: %s", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 1, 2, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "HASH\tCONTENT TYPE\tPATH")
	for _, prefix := range list.CommonPrefixes {
		fmt.Fprintf(w, "%s\t%s\t%s\n", "", "DIR", prefix)
	}
	for _, entry := range list.Entries {
		fmt.Fprintf(w, "%s\t%s\t%s\n", entry.Hash, entry.ContentType, entry.Path)
	}
}
