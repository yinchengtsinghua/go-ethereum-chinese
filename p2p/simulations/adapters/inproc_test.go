
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

package adapters

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/p2p/simulations/pipes"
)

func TestTCPPipe(t *testing.T) {
	c1, c2, err := pipes.TCPPipe()
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})

	go func() {
		msgs := 50
		size := 1024
		for i := 0; i < msgs; i++ {
			msg := make([]byte, size)
			_ = binary.PutUvarint(msg, uint64(i))

			_, err := c1.Write(msg)
			if err != nil {
				t.Fatal(err)
			}
		}

		for i := 0; i < msgs; i++ {
			msg := make([]byte, size)
			_ = binary.PutUvarint(msg, uint64(i))

			out := make([]byte, size)
			_, err := c2.Read(out)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(msg, out) {
				t.Fatalf("expected %#v, got %#v", msg, out)
			}
		}
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("test timeout")
	}
}

func TestTCPPipeBidirections(t *testing.T) {
	c1, c2, err := pipes.TCPPipe()
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})

	go func() {
		msgs := 50
		size := 7
		for i := 0; i < msgs; i++ {
			msg := []byte(fmt.Sprintf("ping %02d", i))

			_, err := c1.Write(msg)
			if err != nil {
				t.Fatal(err)
			}
		}

		for i := 0; i < msgs; i++ {
			expected := []byte(fmt.Sprintf("ping %02d", i))

			out := make([]byte, size)
			_, err := c2.Read(out)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(expected, out) {
				t.Fatalf("expected %#v, got %#v", out, expected)
			} else {
				msg := []byte(fmt.Sprintf("pong %02d", i))
				_, err := c2.Write(msg)
				if err != nil {
					t.Fatal(err)
				}
			}
		}

		for i := 0; i < msgs; i++ {
			expected := []byte(fmt.Sprintf("pong %02d", i))

			out := make([]byte, size)
			_, err := c1.Read(out)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(expected, out) {
				t.Fatalf("expected %#v, got %#v", out, expected)
			}
		}
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("test timeout")
	}
}

func TestNetPipe(t *testing.T) {
	c1, c2, err := pipes.NetPipe()
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})

	go func() {
		msgs := 50
		size := 1024
//网管阻塞，因此写操作是异步发出的。
		go func() {
			for i := 0; i < msgs; i++ {
				msg := make([]byte, size)
				_ = binary.PutUvarint(msg, uint64(i))

				_, err := c1.Write(msg)
				if err != nil {
					t.Fatal(err)
				}
			}
		}()

		for i := 0; i < msgs; i++ {
			msg := make([]byte, size)
			_ = binary.PutUvarint(msg, uint64(i))

			out := make([]byte, size)
			_, err := c2.Read(out)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(msg, out) {
				t.Fatalf("expected %#v, got %#v", msg, out)
			}
		}

		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("test timeout")
	}
}

func TestNetPipeBidirections(t *testing.T) {
	c1, c2, err := pipes.NetPipe()
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})

	go func() {
		msgs := 1000
		size := 8
		pingTemplate := "ping %03d"
		pongTemplate := "pong %03d"

//网管阻塞，因此写操作是异步发出的。
		go func() {
			for i := 0; i < msgs; i++ {
				msg := []byte(fmt.Sprintf(pingTemplate, i))

				_, err := c1.Write(msg)
				if err != nil {
					t.Fatal(err)
				}
			}
		}()

//网管阻塞，因此对pong的读取是异步发出的。
		go func() {
			for i := 0; i < msgs; i++ {
				expected := []byte(fmt.Sprintf(pongTemplate, i))

				out := make([]byte, size)
				_, err := c1.Read(out)
				if err != nil {
					t.Fatal(err)
				}

				if !bytes.Equal(expected, out) {
					t.Fatalf("expected %#v, got %#v", expected, out)
				}
			}

			done <- struct{}{}
		}()

//期望读取ping，并用pong响应备用连接
		for i := 0; i < msgs; i++ {
			expected := []byte(fmt.Sprintf(pingTemplate, i))

			out := make([]byte, size)
			_, err := c2.Read(out)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(expected, out) {
				t.Fatalf("expected %#v, got %#v", expected, out)
			} else {
				msg := []byte(fmt.Sprintf(pongTemplate, i))

				_, err := c2.Write(msg)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("test timeout")
	}
}
