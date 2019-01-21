
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package bn256

import (
	"crypto/rand"

	"testing"
)

func TestLatticeReduceCurve(t *testing.T) {
	k, _ := rand.Int(rand.Reader, Order)
	ks := curveLattice.decompose(k)

	if ks[0].BitLen() > 130 || ks[1].BitLen() > 130 {
		t.Fatal("reduction too large")
	} else if ks[0].Sign() < 0 || ks[1].Sign() < 0 {
		t.Fatal("reduction must be positive")
	}
}

func TestLatticeReduceTarget(t *testing.T) {
	k, _ := rand.Int(rand.Reader, Order)
	ks := targetLattice.decompose(k)

	if ks[0].BitLen() > 66 || ks[1].BitLen() > 66 || ks[2].BitLen() > 66 || ks[3].BitLen() > 66 {
		t.Fatal("reduction too large")
	} else if ks[0].Sign() < 0 || ks[1].Sign() < 0 || ks[2].Sign() < 0 || ks[3].Sign() < 0 {
		t.Fatal("reduction must be positive")
	}
}
