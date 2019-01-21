
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

package trie

import (
	"bytes"
	crand "crypto/rand"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
)

func init() {
	mrand.Seed(time.Now().Unix())
}

//makeprovers根据不同的实现创建merkle trie provers
//测试所有变化。
func makeProvers(trie *Trie) []func(key []byte) *ethdb.MemDatabase {
	var provers []func(key []byte) *ethdb.MemDatabase

//创建直接基于trie的merkle prover
	provers = append(provers, func(key []byte) *ethdb.MemDatabase {
		proof := ethdb.NewMemDatabase()
		trie.Prove(key, 0, proof)
		return proof
	})
//创建基于叶迭代器的merkle prover
	provers = append(provers, func(key []byte) *ethdb.MemDatabase {
		proof := ethdb.NewMemDatabase()
		if it := NewIterator(trie.NodeIterator(key)); it.Next() && bytes.Equal(key, it.Key) {
			for _, p := range it.Prove() {
				proof.Put(crypto.Keccak256(p), p)
			}
		}
		return proof
	})
	return provers
}

func TestProof(t *testing.T) {
	trie, vals := randomTrie(500)
	root := trie.Hash()
	for i, prover := range makeProvers(trie) {
		for _, kv := range vals {
			proof := prover(kv.k)
			if proof == nil {
				t.Fatalf("prover %d: missing key %x while constructing proof", i, kv.k)
			}
			val, _, err := VerifyProof(root, kv.k, proof)
			if err != nil {
				t.Fatalf("prover %d: failed to verify proof for key %x: %v\nraw proof: %x", i, kv.k, err, proof)
			}
			if !bytes.Equal(val, kv.v) {
				t.Fatalf("prover %d: verified value mismatch for key %x: have %x, want %x", i, kv.k, val, kv.v)
			}
		}
	}
}

func TestOneElementProof(t *testing.T) {
	trie := new(Trie)
	updateString(trie, "k", "v")
	for i, prover := range makeProvers(trie) {
		proof := prover([]byte("k"))
		if proof == nil {
			t.Fatalf("prover %d: nil proof", i)
		}
		if proof.Len() != 1 {
			t.Errorf("prover %d: proof should have one element", i)
		}
		val, _, err := VerifyProof(trie.Hash(), []byte("k"), proof)
		if err != nil {
			t.Fatalf("prover %d: failed to verify proof: %v\nraw proof: %x", i, err, proof)
		}
		if !bytes.Equal(val, []byte("v")) {
			t.Fatalf("prover %d: verified value mismatch: have %x, want 'k'", i, val)
		}
	}
}

func TestBadProof(t *testing.T) {
	trie, vals := randomTrie(800)
	root := trie.Hash()
	for i, prover := range makeProvers(trie) {
		for _, kv := range vals {
			proof := prover(kv.k)
			if proof == nil {
				t.Fatalf("prover %d: nil proof", i)
			}
			key := proof.Keys()[mrand.Intn(proof.Len())]
			val, _ := proof.Get(key)
			proof.Delete(key)

			mutateByte(val)
			proof.Put(crypto.Keccak256(val), val)

			if _, _, err := VerifyProof(root, kv.k, proof); err == nil {
				t.Fatalf("prover %d: expected proof to fail for key %x", i, kv.k)
			}
		}
	}
}

//还可以证明缺少密钥的测试。测试显式使用
//条目检索并检查单个条目前后是否有丢失的键。
func TestMissingKeyProof(t *testing.T) {
	trie := new(Trie)
	updateString(trie, "k", "v")

	for i, key := range []string{"a", "j", "l", "z"} {
		proof := ethdb.NewMemDatabase()
		trie.Prove([]byte(key), 0, proof)

		if proof.Len() != 1 {
			t.Errorf("test %d: proof should have one element", i)
		}
		val, _, err := VerifyProof(trie.Hash(), []byte(key), proof)
		if err != nil {
			t.Fatalf("test %d: failed to verify proof: %v\nraw proof: %x", i, err, proof)
		}
		if val != nil {
			t.Fatalf("test %d: verified value mismatch: have %x, want nil", i, val)
		}
	}
}

//突变字节改变了B中的一个字节。
func mutateByte(b []byte) {
	for r := mrand.Intn(len(b)); ; {
		new := byte(mrand.Intn(255))
		if new != b[r] {
			b[r] = new
			break
		}
	}
}

func BenchmarkProve(b *testing.B) {
	trie, vals := randomTrie(100)
	var keys []string
	for k := range vals {
		keys = append(keys, k)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kv := vals[keys[i%len(keys)]]
		proofs := ethdb.NewMemDatabase()
		if trie.Prove(kv.k, 0, proofs); len(proofs.Keys()) == 0 {
			b.Fatalf("zero length proof for %x", kv.k)
		}
	}
}

func BenchmarkVerifyProof(b *testing.B) {
	trie, vals := randomTrie(100)
	root := trie.Hash()
	var keys []string
	var proofs []*ethdb.MemDatabase
	for k := range vals {
		keys = append(keys, k)
		proof := ethdb.NewMemDatabase()
		trie.Prove([]byte(k), 0, proof)
		proofs = append(proofs, proof)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		im := i % len(keys)
		if _, _, err := VerifyProof(root, []byte(keys[im]), proofs[im]); err != nil {
			b.Fatalf("key %x: %v", keys[im], err)
		}
	}
}

func randomTrie(n int) (*Trie, map[string]*kv) {
	trie := new(Trie)
	vals := make(map[string]*kv)
	for i := byte(0); i < 100; i++ {
		value := &kv{common.LeftPadBytes([]byte{i}, 32), []byte{i}, false}
		value2 := &kv{common.LeftPadBytes([]byte{i + 10}, 32), []byte{i}, false}
		trie.Update(value.k, value.v)
		trie.Update(value2.k, value2.v)
		vals[string(value.k)] = value
		vals[string(value2.k)] = value2
	}
	for i := 0; i < n; i++ {
		value := &kv{randBytes(32), randBytes(20), false}
		trie.Update(value.k, value.v)
		vals[string(value.k)] = value
	}
	return trie, vals
}

func randBytes(n int) []byte {
	r := make([]byte, n)
	crand.Read(r)
	return r
}
