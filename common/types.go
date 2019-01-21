
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2015 Go Ethereum作者
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

package common

import (
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"golang.org/x/crypto/sha3"
)

//哈希和地址的长度（字节）。
const (
//hash length是哈希的预期长度
	HashLength = 32
//
	AddressLength = 20
)

var (
	hashT    = reflect.TypeOf(Hash{})
	addressT = reflect.TypeOf(Address{})
)

//hash表示任意数据的32字节keccak256哈希。
type Hash [HashLength]byte

//bytestohash将b设置为hash。
//如果b大于len（h），b将从左侧裁剪。
func BytesToHash(b []byte) Hash {
	var h Hash
	h.SetBytes(b)
	return h
}

//bigtohash将b的字节表示形式设置为hash。
//如果b大于len（h），b将从左侧裁剪。
func BigToHash(b *big.Int) Hash { return BytesToHash(b.Bytes()) }

//hextohash将s的字节表示形式设置为hash。
//如果b大于len（h），b将从左侧裁剪。
func HexToHash(s string) Hash { return BytesToHash(FromHex(s)) }

//Bytes获取基础哈希的字节表示形式。
func (h Hash) Bytes() []byte { return h[:] }

//big将哈希转换为大整数。
func (h Hash) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }

//十六进制将哈希转换为十六进制字符串。
func (h Hash) Hex() string { return hexutil.Encode(h[:]) }

//terminalString实现log.terminalStringer，为控制台格式化字符串
//日志记录期间的输出。
func (h Hash) TerminalString() string {
	return fmt.Sprintf("%x…%x", h[:3], h[29:])
}

//字符串实现Stringer接口，当
//完全登录到文件中。
func (h Hash) String() string {
	return h.Hex()
}

//FORMAT实现fmt.formatter，强制字节片按原样格式化，
//不需要通过用于日志记录的Stringer接口。
func (h Hash) Format(s fmt.State, c rune) {
	fmt.Fprintf(s, "%"+string(c), h[:])
}

//unmarshaltext以十六进制语法分析哈希。
func (h *Hash) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("Hash", input, h[:])
}

//unmarshaljson以十六进制语法解析哈希。
func (h *Hash) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(hashT, input, h[:])
}

//marshalText返回h的十六进制表示形式。
func (h Hash) MarshalText() ([]byte, error) {
	return hexutil.Bytes(h[:]).MarshalText()
}

//setbytes将哈希值设置为b。
//如果b大于len（h），b将从左侧裁剪。
func (h *Hash) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-HashLength:]
	}

	copy(h[HashLength-len(b):], b)
}

//生成工具测试/quick.generator。
func (h Hash) Generate(rand *rand.Rand, size int) reflect.Value {
	m := rand.Intn(len(h))
	for i := len(h) - 1; i > m; i-- {
		h[i] = byte(rand.Uint32())
	}
	return reflect.ValueOf(h)
}

//scan实现数据库/sql的scanner。
func (h *Hash) Scan(src interface{}) error {
	srcB, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("can't scan %T into Hash", src)
	}
	if len(srcB) != HashLength {
		return fmt.Errorf("can't scan []byte of len %d into Hash, want %d", len(srcB), HashLength)
	}
	copy(h[:], srcB)
	return nil
}

//值实现数据库/SQL的值器。
func (h Hash) Value() (driver.Value, error) {
	return h[:], nil
}

//UnprefixedHash允许封送不带0x前缀的哈希。
type UnprefixedHash Hash

//unmashaltext从十六进制解码散列。0x前缀是可选的。
func (h *UnprefixedHash) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedUnprefixedText("UnprefixedHash", input, h[:])
}

//MarshalText将哈希编码为十六进制。
func (h UnprefixedHash) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(h[:])), nil
}

/////////地址

//地址表示以太坊帐户的20字节地址。
type Address [AddressLength]byte

//BytesToAddress返回值为B的地址。
//如果b大于len（h），b将从左侧裁剪。
func BytesToAddress(b []byte) Address {
	var a Address
	a.SetBytes(b)
	return a
}

//BigToAddress返回字节值为B的地址。
//如果b大于len（h），b将从左侧裁剪。
func BigToAddress(b *big.Int) Address { return BytesToAddress(b.Bytes()) }

//hextoAddress返回字节值为s的地址。
//如果s大于len（h），s将从左侧剪切。
func HexToAddress(s string) Address { return BytesToAddress(FromHex(s)) }

//ishexaddress验证字符串是否可以表示有效的十六进制编码
//以太坊地址。
func IsHexAddress(s string) bool {
	if hasHexPrefix(s) {
		s = s[2:]
	}
	return len(s) == 2*AddressLength && isHex(s)
}

//字节获取基础地址的字符串表示形式。
func (a Address) Bytes() []byte { return a[:] }

//big将地址转换为大整数。
func (a Address) Big() *big.Int { return new(big.Int).SetBytes(a[:]) }

//哈希通过左填充零将地址转换为哈希。
func (a Address) Hash() Hash { return BytesToHash(a[:]) }

//十六进制返回地址的符合EIP55的十六进制字符串表示形式。
func (a Address) Hex() string {
	unchecksummed := hex.EncodeToString(a[:])
	sha := sha3.NewLegacyKeccak256()
	sha.Write([]byte(unchecksummed))
	hash := sha.Sum(nil)

	result := []byte(unchecksummed)
	for i := 0; i < len(result); i++ {
		hashByte := hash[i/2]
		if i%2 == 0 {
			hashByte = hashByte >> 4
		} else {
			hashByte &= 0xf
		}
		if result[i] > '9' && hashByte > 7 {
			result[i] -= 32
		}
	}
	return "0x" + string(result)
}

//字符串实现fmt.stringer。
func (a Address) String() string {
	return a.Hex()
}

//FORMAT实现fmt.formatter，强制字节片按原样格式化，
//不需要通过用于日志记录的Stringer接口。
func (a Address) Format(s fmt.State, c rune) {
	fmt.Fprintf(s, "%"+string(c), a[:])
}

//setbytes将地址设置为b的值。
//如果b大于len（a），就会恐慌。
func (a *Address) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-AddressLength:]
	}
	copy(a[AddressLength-len(b):], b)
}

//MarshalText返回的十六进制表示形式。
func (a Address) MarshalText() ([]byte, error) {
	return hexutil.Bytes(a[:]).MarshalText()
}

//unmarshaltext以十六进制语法分析哈希。
func (a *Address) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("Address", input, a[:])
}

//unmarshaljson以十六进制语法解析哈希。
func (a *Address) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(addressT, input, a[:])
}

//scan实现数据库/sql的scanner。
func (a *Address) Scan(src interface{}) error {
	srcB, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("can't scan %T into Address", src)
	}
	if len(srcB) != AddressLength {
		return fmt.Errorf("can't scan []byte of len %d into Address, want %d", len(srcB), AddressLength)
	}
	copy(a[:], srcB)
	return nil
}

//值实现数据库/SQL的值器。
func (a Address) Value() (driver.Value, error) {
	return a[:], nil
}

//UnprefixedAddress允许封送不带0x前缀的地址。
type UnprefixedAddress Address

//unmashaltext从十六进制解码地址。0x前缀是可选的。
func (a *UnprefixedAddress) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedUnprefixedText("UnprefixedAddress", input, a[:])
}

//MarshalText将地址编码为十六进制。
func (a UnprefixedAddress) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(a[:])), nil
}

//mixedcaseaddress保留原始字符串，该字符串可以是也可以不是
//正确校验和
type MixedcaseAddress struct {
	addr     Address
	original string
}

//NewMixedCaseAddress构造函数（主要用于测试）
func NewMixedcaseAddress(addr Address) MixedcaseAddress {
	return MixedcaseAddress{addr: addr, original: addr.Hex()}
}

//newMixedCaseAddressFromString主要用于单元测试
func NewMixedcaseAddressFromString(hexaddr string) (*MixedcaseAddress, error) {
	if !IsHexAddress(hexaddr) {
		return nil, fmt.Errorf("Invalid address")
	}
	a := FromHex(hexaddr)
	return &MixedcaseAddress{addr: BytesToAddress(a), original: hexaddr}, nil
}

//unmashaljson解析mixedcaseaddress
func (ma *MixedcaseAddress) UnmarshalJSON(input []byte) error {
	if err := hexutil.UnmarshalFixedJSON(addressT, input, ma.addr[:]); err != nil {
		return err
	}
	return json.Unmarshal(input, &ma.original)
}

//marshaljson封送原始值
func (ma *MixedcaseAddress) MarshalJSON() ([]byte, error) {
	if strings.HasPrefix(ma.original, "0x") || strings.HasPrefix(ma.original, "0X") {
		return json.Marshal(fmt.Sprintf("0x%s", ma.original[2:]))
	}
	return json.Marshal(fmt.Sprintf("0x%s", ma.original))
}

//地址返回地址
func (ma *MixedcaseAddress) Address() Address {
	return ma.addr
}

//字符串实现fmt.stringer
func (ma *MixedcaseAddress) String() string {
	if ma.ValidChecksum() {
		return fmt.Sprintf("%s [chksum ok]", ma.original)
	}
	return fmt.Sprintf("%s [chksum INVALID]", ma.original)
}

//如果地址具有有效校验和，则valid checksum返回true
func (ma *MixedcaseAddress) ValidChecksum() bool {
	return ma.original == ma.addr.Hex()
}

//original返回混合大小写输入字符串
func (ma *MixedcaseAddress) Original() string {
	return ma.original
}
