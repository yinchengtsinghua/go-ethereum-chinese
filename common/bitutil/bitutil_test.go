
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2013 Go作者。版权所有。
//此源代码的使用受BSD样式的控制
//可以在许可文件中找到的许可证。

//改编自：https://golang.org/src/crypto/cipher/xor_test.go

package bitutil

import (
	"bytes"
	"testing"
)

//测试按位XOR适用于各种对齐。
func TestXOR(t *testing.T) {
	for alignP := 0; alignP < 2; alignP++ {
		for alignQ := 0; alignQ < 2; alignQ++ {
			for alignD := 0; alignD < 2; alignD++ {
				p := make([]byte, 1023)[alignP:]
				q := make([]byte, 1023)[alignQ:]

				for i := 0; i < len(p); i++ {
					p[i] = byte(i)
				}
				for i := 0; i < len(q); i++ {
					q[i] = byte(len(q) - i)
				}
				d1 := make([]byte, 1023+alignD)[alignD:]
				d2 := make([]byte, 1023+alignD)[alignD:]

				XORBytes(d1, p, q)
				safeXORBytes(d2, p, q)
				if !bytes.Equal(d1, d2) {
					t.Error("not equal", d1, d2)
				}
			}
		}
	}
}

//测试按位排列并适用于各种对齐。
func TestAND(t *testing.T) {
	for alignP := 0; alignP < 2; alignP++ {
		for alignQ := 0; alignQ < 2; alignQ++ {
			for alignD := 0; alignD < 2; alignD++ {
				p := make([]byte, 1023)[alignP:]
				q := make([]byte, 1023)[alignQ:]

				for i := 0; i < len(p); i++ {
					p[i] = byte(i)
				}
				for i := 0; i < len(q); i++ {
					q[i] = byte(len(q) - i)
				}
				d1 := make([]byte, 1023+alignD)[alignD:]
				d2 := make([]byte, 1023+alignD)[alignD:]

				ANDBytes(d1, p, q)
				safeANDBytes(d2, p, q)
				if !bytes.Equal(d1, d2) {
					t.Error("not equal")
				}
			}
		}
	}
}

//测试按位或适用于各种对齐的。
func TestOR(t *testing.T) {
	for alignP := 0; alignP < 2; alignP++ {
		for alignQ := 0; alignQ < 2; alignQ++ {
			for alignD := 0; alignD < 2; alignD++ {
				p := make([]byte, 1023)[alignP:]
				q := make([]byte, 1023)[alignQ:]

				for i := 0; i < len(p); i++ {
					p[i] = byte(i)
				}
				for i := 0; i < len(q); i++ {
					q[i] = byte(len(q) - i)
				}
				d1 := make([]byte, 1023+alignD)[alignD:]
				d2 := make([]byte, 1023+alignD)[alignD:]

				ORBytes(d1, p, q)
				safeORBytes(d2, p, q)
				if !bytes.Equal(d1, d2) {
					t.Error("not equal")
				}
			}
		}
	}
}

//位测试适用于各种对齐的测试。
func TestTest(t *testing.T) {
	for align := 0; align < 2; align++ {
//测试批量部件中的位集
		p := make([]byte, 1023)[align:]
		p[100] = 1

		if TestBytes(p) != safeTestBytes(p) {
			t.Error("not equal")
		}
//测试尾部的钻头
		q := make([]byte, 1023)[align:]
		q[len(q)-1] = 1

		if TestBytes(q) != safeTestBytes(q) {
			t.Error("not equal")
		}
	}
}

//对可能优化的XOR性能进行基准测试。
func BenchmarkFastXOR1KB(b *testing.B) { benchmarkFastXOR(b, 1024) }
func BenchmarkFastXOR2KB(b *testing.B) { benchmarkFastXOR(b, 2048) }
func BenchmarkFastXOR4KB(b *testing.B) { benchmarkFastXOR(b, 4096) }

func benchmarkFastXOR(b *testing.B, size int) {
	p, q := make([]byte, size), make([]byte, size)

	for i := 0; i < b.N; i++ {
		XORBytes(p, p, q)
	}
}

//基准测试基线XOR性能。
func BenchmarkBaseXOR1KB(b *testing.B) { benchmarkBaseXOR(b, 1024) }
func BenchmarkBaseXOR2KB(b *testing.B) { benchmarkBaseXOR(b, 2048) }
func BenchmarkBaseXOR4KB(b *testing.B) { benchmarkBaseXOR(b, 4096) }

func benchmarkBaseXOR(b *testing.B, size int) {
	p, q := make([]byte, size), make([]byte, size)

	for i := 0; i < b.N; i++ {
		safeXORBytes(p, p, q)
	}
}

//对潜在的优化和性能进行基准测试。
func BenchmarkFastAND1KB(b *testing.B) { benchmarkFastAND(b, 1024) }
func BenchmarkFastAND2KB(b *testing.B) { benchmarkFastAND(b, 2048) }
func BenchmarkFastAND4KB(b *testing.B) { benchmarkFastAND(b, 4096) }

func benchmarkFastAND(b *testing.B, size int) {
	p, q := make([]byte, size), make([]byte, size)

	for i := 0; i < b.N; i++ {
		ANDBytes(p, p, q)
	}
}

//基准线和绩效。
func BenchmarkBaseAND1KB(b *testing.B) { benchmarkBaseAND(b, 1024) }
func BenchmarkBaseAND2KB(b *testing.B) { benchmarkBaseAND(b, 2048) }
func BenchmarkBaseAND4KB(b *testing.B) { benchmarkBaseAND(b, 4096) }

func benchmarkBaseAND(b *testing.B, size int) {
	p, q := make([]byte, size), make([]byte, size)

	for i := 0; i < b.N; i++ {
		safeANDBytes(p, p, q)
	}
}

//对潜在的优化或性能进行基准测试。
func BenchmarkFastOR1KB(b *testing.B) { benchmarkFastOR(b, 1024) }
func BenchmarkFastOR2KB(b *testing.B) { benchmarkFastOR(b, 2048) }
func BenchmarkFastOR4KB(b *testing.B) { benchmarkFastOR(b, 4096) }

func benchmarkFastOR(b *testing.B, size int) {
	p, q := make([]byte, size), make([]byte, size)

	for i := 0; i < b.N; i++ {
		ORBytes(p, p, q)
	}
}

//基准线或绩效。
func BenchmarkBaseOR1KB(b *testing.B) { benchmarkBaseOR(b, 1024) }
func BenchmarkBaseOR2KB(b *testing.B) { benchmarkBaseOR(b, 2048) }
func BenchmarkBaseOR4KB(b *testing.B) { benchmarkBaseOR(b, 4096) }

func benchmarkBaseOR(b *testing.B, size int) {
	p, q := make([]byte, size), make([]byte, size)

	for i := 0; i < b.N; i++ {
		safeORBytes(p, p, q)
	}
}

//对可能优化的位测试性能进行基准测试。
func BenchmarkFastTest1KB(b *testing.B) { benchmarkFastTest(b, 1024) }
func BenchmarkFastTest2KB(b *testing.B) { benchmarkFastTest(b, 2048) }
func BenchmarkFastTest4KB(b *testing.B) { benchmarkFastTest(b, 4096) }

func benchmarkFastTest(b *testing.B, size int) {
	p := make([]byte, size)
	for i := 0; i < b.N; i++ {
		TestBytes(p)
	}
}

//基准测试基线位测试性能。
func BenchmarkBaseTest1KB(b *testing.B) { benchmarkBaseTest(b, 1024) }
func BenchmarkBaseTest2KB(b *testing.B) { benchmarkBaseTest(b, 2048) }
func BenchmarkBaseTest4KB(b *testing.B) { benchmarkBaseTest(b, 4096) }

func benchmarkBaseTest(b *testing.B, size int) {
	p := make([]byte, size)
	for i := 0; i < b.N; i++ {
		safeTestBytes(p)
	}
}
