
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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

package bitvector

import "testing"

func TestBitvectorNew(t *testing.T) {
	_, err := New(0)
	if err != errInvalidLength {
		t.Errorf("expected err %v, got %v", errInvalidLength, err)
	}

	_, err = NewFromBytes(nil, 0)
	if err != errInvalidLength {
		t.Errorf("expected err %v, got %v", errInvalidLength, err)
	}

	_, err = NewFromBytes([]byte{0}, 9)
	if err != errInvalidLength {
		t.Errorf("expected err %v, got %v", errInvalidLength, err)
	}

	_, err = NewFromBytes(make([]byte, 8), 8)
	if err != nil {
		t.Error(err)
	}
}

func TestBitvectorGetSet(t *testing.T) {
	for _, length := range []int{
		1,
		2,
		4,
		8,
		9,
		15,
		16,
	} {
		bv, err := New(length)
		if err != nil {
			t.Errorf("error for length %v: %v", length, err)
		}

		for i := 0; i < length; i++ {
			if bv.Get(i) {
				t.Errorf("expected false for element on index %v", i)
			}
		}

		func() {
			defer func() {
				if err := recover(); err == nil {
					t.Errorf("expecting panic")
				}
			}()
			bv.Get(length + 8)
		}()

		for i := 0; i < length; i++ {
			bv.Set(i, true)
			for j := 0; j < length; j++ {
				if j == i {
					if !bv.Get(j) {
						t.Errorf("element on index %v is not set to true", i)
					}
				} else {
					if bv.Get(j) {
						t.Errorf("element on index %v is not false", i)
					}
				}
			}

			bv.Set(i, false)

			if bv.Get(i) {
				t.Errorf("element on index %v is not set to false", i)
			}
		}
	}
}

func TestBitvectorNewFromBytesGet(t *testing.T) {
	bv, err := NewFromBytes([]byte{8}, 8)
	if err != nil {
		t.Error(err)
	}
	if !bv.Get(3) {
		t.Fatalf("element 3 is not set to true: state %08b", bv.b[0])
	}
}
