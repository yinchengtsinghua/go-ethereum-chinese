
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2012 Go作者。版权所有。
//此源代码的使用受BSD样式的控制
//可以在许可文件中找到的许可证。

package bn256

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExamplePair(t *testing.T) {
//这实现了从“a”到“a”的三方diffie-hellman算法
//三方diffie-hellman的圆形协议”，A.joux。
//http://www.springerlink.com/content/cddc57yyva0hburb/fulltext.pdf

//三方（A、B和C）中的每一方都会产生一个私人价值。
	a, _ := rand.Int(rand.Reader, Order)
	b, _ := rand.Int(rand.Reader, Order)
	c, _ := rand.Int(rand.Reader, Order)

//然后，每一方计算g_和g₂乘以其私有价值。
	pa := new(G1).ScalarBaseMult(a)
	qa := new(G2).ScalarBaseMult(a)

	pb := new(G1).ScalarBaseMult(b)
	qb := new(G2).ScalarBaseMult(b)

	pc := new(G1).ScalarBaseMult(c)
	qc := new(G2).ScalarBaseMult(c)

//现在，每一方都与另外两方交换其公共价值观，以及
//所有参与方都可以计算共享密钥。
	k1 := Pair(pb, qc)
	k1.ScalarMult(k1, a)

	k2 := Pair(pc, qa)
	k2.ScalarMult(k2, b)

	k3 := Pair(pa, qb)
	k3.ScalarMult(k3, c)

//k1、k2和k3都相等。

	require.Equal(t, k1, k2)
	require.Equal(t, k1, k3)

require.Equal(t, len(np), 4) //避免NP上的gometalinter varcheck错误
}
