
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

//包含公用包中的所有包装器。

package geth

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

//hash表示任意数据的32字节keccak256哈希。
type Hash struct {
	hash common.Hash
}

//NewHashFromBytes将字节切片转换为哈希值。
func NewHashFromBytes(binary []byte) (hash *Hash, _ error) {
	h := new(Hash)
	if err := h.SetBytes(common.CopyBytes(binary)); err != nil {
		return nil, err
	}
	return h, nil
}

//NewHashFromHex将十六进制字符串转换为哈希值。
func NewHashFromHex(hex string) (hash *Hash, _ error) {
	h := new(Hash)
	if err := h.SetHex(hex); err != nil {
		return nil, err
	}
	return h, nil
}

//setbytes将指定的字节切片设置为哈希值。
func (h *Hash) SetBytes(hash []byte) error {
	if length := len(hash); length != common.HashLength {
		return fmt.Errorf("invalid hash length: %v != %v", length, common.HashLength)
	}
	copy(h.hash[:], hash)
	return nil
}

//GetBytes检索哈希的字节表示形式。
func (h *Hash) GetBytes() []byte {
	return h.hash[:]
}

//sethex将指定的十六进制字符串设置为哈希值。
func (h *Hash) SetHex(hash string) error {
	hash = strings.ToLower(hash)
	if len(hash) >= 2 && hash[:2] == "0x" {
		hash = hash[2:]
	}
	if length := len(hash); length != 2*common.HashLength {
		return fmt.Errorf("invalid hash hex length: %v != %v", length, 2*common.HashLength)
	}
	bin, err := hex.DecodeString(hash)
	if err != nil {
		return err
	}
	copy(h.hash[:], bin)
	return nil
}

//gethex检索哈希的十六进制字符串表示形式。
func (h *Hash) GetHex() string {
	return h.hash.Hex()
}

//哈希表示哈希的一部分。
type Hashes struct{ hashes []common.Hash }

//newhashes创建未初始化哈希的切片。
func NewHashes(size int) *Hashes {
	return &Hashes{
		hashes: make([]common.Hash, size),
	}
}

//newhashempty创建哈希值的空切片。
func NewHashesEmpty() *Hashes {
	return NewHashes(0)
}

//SIZE返回切片中的哈希数。
func (h *Hashes) Size() int {
	return len(h.hashes)
}

//get返回切片中给定索引处的哈希。
func (h *Hashes) Get(index int) (hash *Hash, _ error) {
	if index < 0 || index >= len(h.hashes) {
		return nil, errors.New("index out of bounds")
	}
	return &Hash{h.hashes[index]}, nil
}

//set在切片中的给定索引处设置哈希。
func (h *Hashes) Set(index int, hash *Hash) error {
	if index < 0 || index >= len(h.hashes) {
		return errors.New("index out of bounds")
	}
	h.hashes[index] = hash.hash
	return nil
}

//append在切片的末尾添加一个新的哈希元素。
func (h *Hashes) Append(hash *Hash) {
	h.hashes = append(h.hashes, hash.hash)
}

//地址表示以太坊帐户的20字节地址。
type Address struct {
	address common.Address
}

//newAddressFromBytes将字节切片转换为哈希值。
func NewAddressFromBytes(binary []byte) (address *Address, _ error) {
	a := new(Address)
	if err := a.SetBytes(common.CopyBytes(binary)); err != nil {
		return nil, err
	}
	return a, nil
}

//newAddressFromHex将十六进制字符串转换为地址值。
func NewAddressFromHex(hex string) (address *Address, _ error) {
	a := new(Address)
	if err := a.SetHex(hex); err != nil {
		return nil, err
	}
	return a, nil
}

//setbytes将指定的字节片设置为地址值。
func (a *Address) SetBytes(address []byte) error {
	if length := len(address); length != common.AddressLength {
		return fmt.Errorf("invalid address length: %v != %v", length, common.AddressLength)
	}
	copy(a.address[:], address)
	return nil
}

//GetBytes检索地址的字节表示形式。
func (a *Address) GetBytes() []byte {
	return a.address[:]
}

//sethex将指定的十六进制字符串设置为地址值。
func (a *Address) SetHex(address string) error {
	address = strings.ToLower(address)
	if len(address) >= 2 && address[:2] == "0x" {
		address = address[2:]
	}
	if length := len(address); length != 2*common.AddressLength {
		return fmt.Errorf("invalid address hex length: %v != %v", length, 2*common.AddressLength)
	}
	bin, err := hex.DecodeString(address)
	if err != nil {
		return err
	}
	copy(a.address[:], bin)
	return nil
}

//gethex检索地址的十六进制字符串表示形式。
func (a *Address) GetHex() string {
	return a.address.Hex()
}

//地址表示一个地址片。
type Addresses struct{ addresses []common.Address }

//newaddresses创建一个未初始化的地址切片。
func NewAddresses(size int) *Addresses {
	return &Addresses{
		addresses: make([]common.Address, size),
	}
}

//newAddressEmpty创建地址值的空切片。
func NewAddressesEmpty() *Addresses {
	return NewAddresses(0)
}

//SIZE返回切片中的地址数。
func (a *Addresses) Size() int {
	return len(a.addresses)
}

//GET从切片返回给定索引处的地址。
func (a *Addresses) Get(index int) (address *Address, _ error) {
	if index < 0 || index >= len(a.addresses) {
		return nil, errors.New("index out of bounds")
	}
	return &Address{a.addresses[index]}, nil
}

//set设置切片中给定索引的地址。
func (a *Addresses) Set(index int, address *Address) error {
	if index < 0 || index >= len(a.addresses) {
		return errors.New("index out of bounds")
	}
	a.addresses[index] = address.address
	return nil
}

//append将新的地址元素添加到切片的末尾。
func (a *Addresses) Append(address *Address) {
	a.addresses = append(a.addresses, address.address)
}
