
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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

package testutil

import (
	"bytes"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"testing"
)

//tempfilewithcontent是一个助手函数，它创建一个包含以下字符串内容的临时文件，然后关闭文件句柄
//它返回完整的文件路径
func TempFileWithContent(t *testing.T, content string) string {
	tempFile, err := ioutil.TempFile("", "swarm-temp-file")
	if err != nil {
		t.Fatal(err)
	}

	_, err = io.Copy(tempFile, strings.NewReader(content))
	if err != nil {
		os.RemoveAll(tempFile.Name())
		t.Fatal(err)
	}
	if err = tempFile.Close(); err != nil {
		t.Fatal(err)
	}
	return tempFile.Name()
}

//RandomBytes返回伪随机确定性结果
//因为测试失败必须是可复制的
func RandomBytes(seed, length int) []byte {
	b := make([]byte, length)
	reader := rand.New(rand.NewSource(int64(seed)))
	for n := 0; n < length; {
		read, err := reader.Read(b[n:])
		if err != nil {
			panic(err)
		}
		n += read
	}
	return b
}

func RandomReader(seed, length int) *bytes.Reader {
	return bytes.NewReader(RandomBytes(seed, length))
}
