
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//+构建GO1.8
//
//版权所有2018 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//Go-Ethereum库是免费软件：您可以重新分发它和/或修改
//根据GNU发布的较低通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊图书馆的发行目的是希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

package db

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/swarm/storage/mock/test"
)

//testdbstore正在运行test.mockstore测试
//使用test.mockstore函数。
func TestDBStore(t *testing.T) {
	dir, err := ioutil.TempDir("", "mock_"+t.Name())
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	store, err := NewGlobalStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	test.MockStore(t, store, 100)
}

//testmortexport正在运行test.importexport测试
//使用test.mockstore函数。
func TestImportExport(t *testing.T) {
	dir1, err := ioutil.TempDir("", "mock_"+t.Name()+"_exporter")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir1)

	store1, err := NewGlobalStore(dir1)
	if err != nil {
		t.Fatal(err)
	}
	defer store1.Close()

	dir2, err := ioutil.TempDir("", "mock_"+t.Name()+"_importer")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir2)

	store2, err := NewGlobalStore(dir2)
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	test.ImportExport(t, store1, store2, 100)
}
