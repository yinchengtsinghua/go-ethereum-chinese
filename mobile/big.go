
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

//包含来自Math/Big包的所有包装。

package geth

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

//bigint表示带符号的多精度整数。
type BigInt struct {
	bigint *big.Int
}

//new bigint分配并返回一个新的bigint集到x。
func NewBigInt(x int64) *BigInt {
	return &BigInt{big.NewInt(x)}
}

//GetBytes返回x的绝对值作为一个大尾数字节片。
func (bi *BigInt) GetBytes() []byte {
	return bi.bigint.Bytes()
}

//字符串以格式化的十进制字符串形式返回x的值。
func (bi *BigInt) String() string {
	return bi.bigint.String()
}

//GetInt64返回x的Int64表示形式。如果x不能用
//一个Int64，结果是未定义的。
func (bi *BigInt) GetInt64() int64 {
	return bi.bigint.Int64()
}

//setbytes将buf解释为big endian无符号整数的字节，并设置
//这个值的整数。
func (bi *BigInt) SetBytes(buf []byte) {
	bi.bigint.SetBytes(common.CopyBytes(buf))
}

//setInt64将大int设置为x。
func (bi *BigInt) SetInt64(x int64) {
	bi.bigint.SetInt64(x)
}

//签名返回：
//
//-如果x＜1则为0
//0如果x＝＝0
//+ 1如果x＞0
//
func (bi *BigInt) Sign() int {
	return bi.bigint.Sign()
}

//setString将big int设置为x。
//
//字符串前缀决定了实际的转换基数。前缀“0x”或
//“0x”选择基数16；“0”前缀选择基数8，“0b”或“0b”前缀
//选择基数2。否则，选择的基数为10。
func (bi *BigInt) SetString(x string, base int) {
	bi.bigint.SetString(x, base)
}

//big ints代表一部分大整数。
type BigInts struct{ bigints []*big.Int }

//新手创建了一块未初始化的大数字。
func NewBigInts(size int) *BigInts {
	return &BigInts{
		bigints: make([]*big.Int, size),
	}
}

//SIZE返回切片中的大整数数。
func (bi *BigInts) Size() int {
	return len(bi.bigints)
}

//get从切片返回给定索引处的bigint。
func (bi *BigInts) Get(index int) (bigint *BigInt, _ error) {
	if index < 0 || index >= len(bi.bigints) {
		return nil, errors.New("index out of bounds")
	}
	return &BigInt{bi.bigints[index]}, nil
}

//set在切片中的给定索引处设置big int。
func (bi *BigInts) Set(index int, bigint *BigInt) error {
	if index < 0 || index >= len(bi.bigints) {
		return errors.New("index out of bounds")
	}
	bi.bigints[index] = bigint.bigint
	return nil
}

//GetString返回x的值，该值是以某个数字为基数的格式化字符串。
func (bi *BigInt) GetString(base int) string {
	return bi.bigint.Text(base)
}
