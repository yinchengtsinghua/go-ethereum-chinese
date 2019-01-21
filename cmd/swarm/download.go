
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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/swarm/api"
	swarm "github.com/ethereum/go-ethereum/swarm/api/client"
	"gopkg.in/urfave/cli.v1"
)

var downloadCommand = cli.Command{
	Action:      download,
	Name:        "down",
	Flags:       []cli.Flag{SwarmRecursiveFlag, SwarmAccessPasswordFlag},
	Usage:       "downloads a swarm manifest or a file inside a manifest",
	ArgsUsage:   " <uri> [<dir>]",
	Description: `Downloads a swarm bzz uri to the given dir. When no dir is provided, working directory is assumed. --recursive flag is expected when downloading a manifest with multiple entries.`,
}

func download(ctx *cli.Context) {
	log.Debug("downloading content using swarm down")
	args := ctx.Args()
	dest := "."

	switch len(args) {
	case 0:
		utils.Fatalf("Usage: swarm down [options] <bzz locator> [<destination path>]")
	case 1:
		log.Trace(fmt.Sprintf("swarm down: no destination path - assuming working dir"))
	default:
		log.Trace(fmt.Sprintf("destination path arg: %s", args[1]))
		if absDest, err := filepath.Abs(args[1]); err == nil {
			dest = absDest
		} else {
			utils.Fatalf("could not get download path: %v", err)
		}
	}

	var (
		bzzapi      = strings.TrimRight(ctx.GlobalString(SwarmApiFlag.Name), "/")
		isRecursive = ctx.Bool(SwarmRecursiveFlag.Name)
		client      = swarm.NewClient(bzzapi)
	)

	if fi, err := os.Stat(dest); err == nil {
		if isRecursive && !fi.Mode().IsDir() {
			utils.Fatalf("destination path is not a directory!")
		}
	} else {
		if !os.IsNotExist(err) {
			utils.Fatalf("could not stat path: %v", err)
		}
	}

	uri, err := api.Parse(args[0])
	if err != nil {
		utils.Fatalf("could not parse uri argument: %v", err)
	}

	dl := func(credentials string) error {
//
		if isRecursive {
			if err := client.DownloadDirectory(uri.Addr, uri.Path, dest, credentials); err != nil {
				if err == swarm.ErrUnauthorized {
					return err
				}
				return fmt.Errorf("directory %s: %v", uri.Path, err)
			}
		} else {
//
			log.Debug("downloading file/path from a manifest", "uri.Addr", uri.Addr, "uri.Path", uri.Path)

			err := client.DownloadFile(uri.Addr, uri.Path, dest, credentials)
			if err != nil {
				if err == swarm.ErrUnauthorized {
					return err
				}
				return fmt.Errorf("file %s from address: %s: %v", uri.Path, uri.Addr, err)
			}
		}
		return nil
	}
	if passwords := makePasswordList(ctx); passwords != nil {
		password := getPassPhrase(fmt.Sprintf("Downloading %s is restricted", uri), 0, passwords)
		err = dl(password)
	} else {
		err = dl("")
	}
	if err != nil {
		utils.Fatalf("download: %v", err)
	}
}
