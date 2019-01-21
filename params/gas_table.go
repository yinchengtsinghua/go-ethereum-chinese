
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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

package params

//Gastable组织不同以太坊阶段的天然气价格。
type GasTable struct {
	ExtcodeSize uint64
	ExtcodeCopy uint64
	ExtcodeHash uint64
	Balance     uint64
	SLoad       uint64
	Calls       uint64
	Suicide     uint64

	ExpByte uint64

//当
//退款账户是这样的
//不存在。这种逻辑是相似的
//打电话。可以留空。零均值
//不收费。
	CreateBySuicide uint64
}

//包含不同以太坊阶段天然气价格的变量。
var (
//GastableHomestead包含
//宅基地阶段。
	GasTableHomestead = GasTable{
		ExtcodeSize: 20,
		ExtcodeCopy: 20,
		Balance:     20,
		SLoad:       50,
		Calls:       40,
		Suicide:     0,
		ExpByte:     10,
	}

//GastableEIP150包含天然气再定价
//EIP150相位。
	GasTableEIP150 = GasTable{
		ExtcodeSize: 700,
		ExtcodeCopy: 700,
		Balance:     400,
		SLoad:       200,
		Calls:       700,
		Suicide:     5000,
		ExpByte:     10,

		CreateBySuicide: 25000,
	}
//GastableEIP158包含
//EIP155/EIP158阶段。
	GasTableEIP158 = GasTable{
		ExtcodeSize: 700,
		ExtcodeCopy: 700,
		Balance:     400,
		SLoad:       200,
		Calls:       700,
		Suicide:     5000,
		ExpByte:     50,

		CreateBySuicide: 25000,
	}
//天然气君士坦丁堡包含天然气再定价
//君士坦丁堡时期。
	GasTableConstantinople = GasTable{
		ExtcodeSize: 700,
		ExtcodeCopy: 700,
		ExtcodeHash: 400,
		Balance:     400,
		SLoad:       200,
		Calls:       700,
		Suicide:     5000,
		ExpByte:     50,

		CreateBySuicide: 25000,
	}
)
