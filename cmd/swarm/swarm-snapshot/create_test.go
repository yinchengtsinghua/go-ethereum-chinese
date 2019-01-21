
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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/p2p/simulations"
)

//testsnapshotcreate是一个高级别的e2e测试，用于测试快照生成。
//它运行一些带有不同标志值和生成的加载的“创建”命令
//快照文件以验证其内容。
func TestSnapshotCreate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	for _, v := range []struct {
		name     string
		nodes    int
		services string
	}{
		{
			name: "defaults",
		},
		{
			name:  "more nodes",
			nodes: defaultNodes + 5,
		},
		{
			name:     "services",
			services: "stream,pss,zorglub",
		},
		{
			name:     "services with bzz",
			services: "bzz,pss",
		},
	} {
		t.Run(v.name, func(t *testing.T) {
			t.Parallel()

			file, err := ioutil.TempFile("", "swarm-snapshot")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(file.Name())

			if err = file.Close(); err != nil {
				t.Error(err)
			}

			args := []string{"create"}
			if v.nodes > 0 {
				args = append(args, "--nodes", strconv.Itoa(v.nodes))
			}
			if v.services != "" {
				args = append(args, "--services", v.services)
			}
			testCmd := runSnapshot(t, append(args, file.Name())...)

			testCmd.ExpectExit()
			if code := testCmd.ExitStatus(); code != 0 {
				t.Fatalf("command exit code %v, expected 0", code)
			}

			f, err := os.Open(file.Name())
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				err := f.Close()
				if err != nil {
					t.Error("closing snapshot file", "err", err)
				}
			}()

			b, err := ioutil.ReadAll(f)
			if err != nil {
				t.Fatal(err)
			}
			var snap simulations.Snapshot
			err = json.Unmarshal(b, &snap)
			if err != nil {
				t.Fatal(err)
			}

			wantNodes := v.nodes
			if wantNodes == 0 {
				wantNodes = defaultNodes
			}
			gotNodes := len(snap.Nodes)
			if gotNodes != wantNodes {
				t.Errorf("got %v nodes, want %v", gotNodes, wantNodes)
			}

			if len(snap.Conns) == 0 {
				t.Error("no connections in a snapshot")
			}

			var wantServices []string
			if v.services != "" {
				wantServices = strings.Split(v.services, ",")
			} else {
				wantServices = []string{"bzz"}
			}
//对服务名进行排序，以便进行比较
//作为每个节点排序服务的字符串
			sort.Strings(wantServices)

			for i, n := range snap.Nodes {
				gotServices := n.Node.Config.Services
				sort.Strings(gotServices)
				if fmt.Sprint(gotServices) != fmt.Sprint(wantServices) {
					t.Errorf("got services %v for node %v, want %v", gotServices, i, wantServices)
				}
			}

		})
	}
}
