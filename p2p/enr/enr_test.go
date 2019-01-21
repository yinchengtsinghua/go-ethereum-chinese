
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

package enr

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var rnd = rand.New(rand.NewSource(time.Now().UnixNano()))

func randomString(strlen int) string {
	b := make([]byte, strlen)
	rnd.Read(b)
	return string(b)
}

//testgetsetid测试ID键的编码/解码和设置/获取。
func TestGetSetID(t *testing.T) {
	id := ID("someid")
	var r Record
	r.Set(id)

	var id2 ID
	require.NoError(t, r.Load(&id2))
	assert.Equal(t, id, id2)
}

//testgetsetip4测试IP密钥的编码/解码和设置/获取。
func TestGetSetIP4(t *testing.T) {
	ip := IP{192, 168, 0, 3}
	var r Record
	r.Set(ip)

	var ip2 IP
	require.NoError(t, r.Load(&ip2))
	assert.Equal(t, ip, ip2)
}

//testgetsetip6测试IP密钥的编码/解码和设置/获取。
func TestGetSetIP6(t *testing.T) {
	ip := IP{0x20, 0x01, 0x48, 0x60, 0, 0, 0x20, 0x01, 0, 0, 0, 0, 0, 0, 0x00, 0x68}
	var r Record
	r.Set(ip)

	var ip2 IP
	require.NoError(t, r.Load(&ip2))
	assert.Equal(t, ip, ip2)
}

//testgetsetdiscport测试discport密钥的编码/解码和设置/获取。
func TestGetSetUDP(t *testing.T) {
	port := UDP(30309)
	var r Record
	r.Set(port)

	var port2 UDP
	require.NoError(t, r.Load(&port2))
	assert.Equal(t, port, port2)
}

func TestLoadErrors(t *testing.T) {
	var r Record
	ip4 := IP{127, 0, 0, 1}
	r.Set(ip4)

//检查是否有钥匙丢失的错误。
	var udp UDP
	err := r.Load(&udp)
	if !IsNotFound(err) {
		t.Error("IsNotFound should return true for missing key")
	}
	assert.Equal(t, &KeyError{Key: udp.ENRKey(), Err: errNotFound}, err)

//检查无效密钥的错误。
	var list []uint
	err = r.Load(WithEntry(ip4.ENRKey(), &list))
	kerr, ok := err.(*KeyError)
	if !ok {
		t.Fatalf("expected KeyError, got %T", err)
	}
	assert.Equal(t, kerr.Key, ip4.ENRKey())
	assert.Error(t, kerr.Err)
	if IsNotFound(err) {
		t.Error("IsNotFound should return false for decoding errors")
	}
}

//testsortedgetandset生成排序对切片的测试。
func TestSortedGetAndSet(t *testing.T) {
	type pair struct {
		k string
		v uint32
	}

	for _, tt := range []struct {
		input []pair
		want  []pair
	}{
		{
			input: []pair{{"a", 1}, {"c", 2}, {"b", 3}},
			want:  []pair{{"a", 1}, {"b", 3}, {"c", 2}},
		},
		{
			input: []pair{{"a", 1}, {"c", 2}, {"b", 3}, {"d", 4}, {"a", 5}, {"bb", 6}},
			want:  []pair{{"a", 5}, {"b", 3}, {"bb", 6}, {"c", 2}, {"d", 4}},
		},
		{
			input: []pair{{"c", 2}, {"b", 3}, {"d", 4}, {"a", 5}, {"bb", 6}},
			want:  []pair{{"a", 5}, {"b", 3}, {"bb", 6}, {"c", 2}, {"d", 4}},
		},
	} {
		var r Record
		for _, i := range tt.input {
			r.Set(WithEntry(i.k, &i.v))
		}
		for i, w := range tt.want {
//设置r.pair[i]中的got's键，以便保留对的顺序
			got := pair{k: r.pairs[i].k}
			assert.NoError(t, r.Load(WithEntry(w.k, &got.v)))
			assert.Equal(t, w, got)
		}
	}
}

//testDirty测试在记录中设置新的键/值对时记录签名删除。
func TestDirty(t *testing.T) {
	var r Record

	if _, err := rlp.EncodeToBytes(r); err != errEncodeUnsigned {
		t.Errorf("expected errEncodeUnsigned, got %#v", err)
	}

	require.NoError(t, signTest([]byte{5}, &r))
	if len(r.signature) == 0 {
		t.Error("record is not signed")
	}
	_, err := rlp.EncodeToBytes(r)
	assert.NoError(t, err)

	r.SetSeq(3)
	if len(r.signature) != 0 {
		t.Error("signature still set after modification")
	}
	if _, err := rlp.EncodeToBytes(r); err != errEncodeUnsigned {
		t.Errorf("expected errEncodeUnsigned, got %#v", err)
	}
}

func TestSeq(t *testing.T) {
	var r Record

	assert.Equal(t, uint64(0), r.Seq())
	r.Set(UDP(1))
	assert.Equal(t, uint64(0), r.Seq())
	signTest([]byte{5}, &r)
	assert.Equal(t, uint64(0), r.Seq())
	r.Set(UDP(2))
	assert.Equal(t, uint64(1), r.Seq())
}

//使用记录中的现有键设置新值时，testgetsetoverwrite测试值overwrite。
func TestGetSetOverwrite(t *testing.T) {
	var r Record

	ip := IP{192, 168, 0, 3}
	r.Set(ip)

	ip2 := IP{192, 168, 0, 4}
	r.Set(ip2)

	var ip3 IP
	require.NoError(t, r.Load(&ip3))
	assert.Equal(t, ip2, ip3)
}

//testsignencodeanddecode测试记录的签名、rlp编码和rlp解码。
func TestSignEncodeAndDecode(t *testing.T) {
	var r Record
	r.Set(UDP(30303))
	r.Set(IP{127, 0, 0, 1})
	require.NoError(t, signTest([]byte{5}, &r))

	blob, err := rlp.EncodeToBytes(r)
	require.NoError(t, err)

	var r2 Record
	require.NoError(t, rlp.DecodeBytes(blob, &r2))
	assert.Equal(t, r, r2)

	blob2, err := rlp.EncodeToBytes(r2)
	require.NoError(t, err)
	assert.Equal(t, blob, blob2)
}

//无法对记录大于sizelimit字节的TestRecordToObig测试进行签名。
func TestRecordTooBig(t *testing.T) {
	var r Record
	key := randomString(10)

//为随机键设置大值，预计错误
	r.Set(WithEntry(key, randomString(SizeLimit)))
	if err := signTest([]byte{5}, &r); err != errTooBig {
		t.Fatalf("expected to get errTooBig, got %#v", err)
	}

//为随机键设置一个可接受的值，不期望出现错误
	r.Set(WithEntry(key, randomString(100)))
	require.NoError(t, signTest([]byte{5}, &r))
}

//testsignencodeanddecoderandom测试包含随机键/值对的记录的编码/解码。
func TestSignEncodeAndDecodeRandom(t *testing.T) {
	var r Record

//用于测试的随机键/值对
	pairs := map[string]uint32{}
	for i := 0; i < 10; i++ {
		key := randomString(7)
		value := rnd.Uint32()
		pairs[key] = value
		r.Set(WithEntry(key, &value))
	}

	require.NoError(t, signTest([]byte{5}, &r))
	_, err := rlp.EncodeToBytes(r)
	require.NoError(t, err)

	for k, v := range pairs {
		desc := fmt.Sprintf("key %q", k)
		var got uint32
		buf := WithEntry(k, &got)
		require.NoError(t, r.Load(buf), desc)
		require.Equal(t, v, got, desc)
	}
}

type testSig struct{}

type testID []byte

func (id testID) ENRKey() string { return "testid" }

func signTest(id []byte, r *Record) error {
	r.Set(ID("test"))
	r.Set(testID(id))
	return r.SetSig(testSig{}, makeTestSig(id, r.Seq()))
}

func makeTestSig(id []byte, seq uint64) []byte {
	sig := make([]byte, 8, len(id)+8)
	binary.BigEndian.PutUint64(sig[:8], seq)
	sig = append(sig, id...)
	return sig
}

func (testSig) Verify(r *Record, sig []byte) error {
	var id []byte
	if err := r.Load((*testID)(&id)); err != nil {
		return err
	}
	if !bytes.Equal(sig, makeTestSig(id, r.Seq())) {
		return ErrInvalidSig
	}
	return nil
}

func (testSig) NodeAddr(r *Record) []byte {
	var id []byte
	if err := r.Load((*testID)(&id)); err != nil {
		return nil
	}
	return id
}
