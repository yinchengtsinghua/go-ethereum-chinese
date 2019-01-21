
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 P_ter Szil_gyi。版权所有。
//此源代码的使用受可以找到的BSD样式许可证的控制
//在许可证文件中。

//+构建GouuZZ

package bn256

import (
	"bytes"
	"math/big"

	cloudflare "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"
	google "github.com/ethereum/go-ethereum/crypto/bn256/google"
)

//在Google和CloudFlare库之间添加了fuzzaddfuzzzbn256。
func FuzzAdd(data []byte) int {
//首先确保我们有足够的数据
	if len(data) != 128 {
		return 0
	}
//确保两个libs都能解析第一个曲线点
	xc := new(cloudflare.G1)
	_, errc := xc.Unmarshal(data[:64])

	xg := new(google.G1)
	_, errg := xg.Unmarshal(data[:64])

	if (errc == nil) != (errg == nil) {
		panic("parse mismatch")
	} else if errc != nil {
		return 0
	}
//确保两个libs都能解析第二个曲线点
	yc := new(cloudflare.G1)
	_, errc = yc.Unmarshal(data[64:])

	yg := new(google.G1)
	_, errg = yg.Unmarshal(data[64:])

	if (errc == nil) != (errg == nil) {
		panic("parse mismatch")
	} else if errc != nil {
		return 0
	}
//将这两个点相加，确保结果相同
	rc := new(cloudflare.G1)
	rc.Add(xc, yc)

	rg := new(google.G1)
	rg.Add(xg, yg)

	if !bytes.Equal(rc.Marshal(), rg.Marshal()) {
		panic("add mismatch")
	}
	return 0
}

//Google和CloudFlare之间的fuzzmul fuzzez bn256标量乘法
//图书馆。
func FuzzMul(data []byte) int {
//首先确保我们有足够的数据
	if len(data) != 96 {
		return 0
	}
//确保两个libs都能解析曲线点
	pc := new(cloudflare.G1)
	_, errc := pc.Unmarshal(data[:64])

	pg := new(google.G1)
	_, errg := pg.Unmarshal(data[:64])

	if (errc == nil) != (errg == nil) {
		panic("parse mismatch")
	} else if errc != nil {
		return 0
	}
//将这两个点相加，确保结果相同
	rc := new(cloudflare.G1)
	rc.ScalarMult(pc, new(big.Int).SetBytes(data[64:]))

	rg := new(google.G1)
	rg.ScalarMult(pg, new(big.Int).SetBytes(data[64:]))

	if !bytes.Equal(rc.Marshal(), rg.Marshal()) {
		panic("scalar mul mismatch")
	}
	return 0
}

func FuzzPair(data []byte) int {
//首先确保我们有足够的数据
	if len(data) != 192 {
		return 0
	}
//确保两个libs都能解析曲线点
	pc := new(cloudflare.G1)
	_, errc := pc.Unmarshal(data[:64])

	pg := new(google.G1)
	_, errg := pg.Unmarshal(data[:64])

	if (errc == nil) != (errg == nil) {
		panic("parse mismatch")
	} else if errc != nil {
		return 0
	}
//确保两个libs都能解析转折点
	tc := new(cloudflare.G2)
	_, errc = tc.Unmarshal(data[64:])

	tg := new(google.G2)
	_, errg = tg.Unmarshal(data[64:])

	if (errc == nil) != (errg == nil) {
		panic("parse mismatch")
	} else if errc != nil {
		return 0
	}
//将两个点配对，确保结果相同
	if cloudflare.PairingCheck([]*cloudflare.G1{pc}, []*cloudflare.G2{tc}) != google.PairingCheck([]*google.G1{pg}, []*google.G2{tg}) {
		panic("pair mismatch")
	}
	return 0
}
