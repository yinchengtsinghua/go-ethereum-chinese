
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

package netutil

import (
	"fmt"
	"net"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/davecgh/go-spew/spew"
)

func TestParseNetlist(t *testing.T) {
	var tests = []struct {
		input    string
		wantErr  error
		wantList *Netlist
	}{
		{
			input:    "",
			wantList: &Netlist{},
		},
		{
			input:    "127.0.0.0/8",
			wantErr:  nil,
			wantList: &Netlist{{IP: net.IP{127, 0, 0, 0}, Mask: net.CIDRMask(8, 32)}},
		},
		{
			input:   "127.0.0.0/44",
			wantErr: &net.ParseError{Type: "CIDR address", Text: "127.0.0.0/44"},
		},
		{
			input: "127.0.0.0/16, 23.23.23.23/24,",
			wantList: &Netlist{
				{IP: net.IP{127, 0, 0, 0}, Mask: net.CIDRMask(16, 32)},
				{IP: net.IP{23, 23, 23, 0}, Mask: net.CIDRMask(24, 32)},
			},
		},
	}

	for _, test := range tests {
		l, err := ParseNetlist(test.input)
		if !reflect.DeepEqual(err, test.wantErr) {
			t.Errorf("%q: got error %q, want %q", test.input, err, test.wantErr)
			continue
		}
		if !reflect.DeepEqual(l, test.wantList) {
			spew.Dump(l)
			spew.Dump(test.wantList)
			t.Errorf("%q: got %v, want %v", test.input, l, test.wantList)
		}
	}
}

func TestNilNetListContains(t *testing.T) {
	var list *Netlist
	checkContains(t, list.Contains, nil, []string{"1.2.3.4"})
}

func TestIsLAN(t *testing.T) {
	checkContains(t, IsLAN,
[]string{ //包括
			"0.0.0.0",
			"0.2.0.8",
			"127.0.0.1",
			"10.0.1.1",
			"10.22.0.3",
			"172.31.252.251",
			"192.168.1.4",
			"fe80::f4a1:8eff:fec5:9d9d",
			"febf::ab32:2233",
			"fc00::4",
		},
[]string{ //排除
			"192.0.2.1",
			"1.0.0.0",
			"172.32.0.1",
			"fec0::2233",
		},
	)
}

func TestIsSpecialNetwork(t *testing.T) {
	checkContains(t, IsSpecialNetwork,
[]string{ //包括
			"192.0.2.1",
			"192.0.2.44",
			"2001:db8:85a3:8d3:1319:8a2e:370:7348",
			"255.255.255.255",
"224.0.0.22", //IPv4组播
"ff05::1:3",  //IPv6组播
		},
[]string{ //排除
			"192.0.3.1",
			"1.0.0.0",
			"172.32.0.1",
			"fec0::2233",
		},
	)
}

func checkContains(t *testing.T, fn func(net.IP) bool, inc, exc []string) {
	for _, s := range inc {
		if !fn(parseIP(s)) {
			t.Error("returned false for included address", s)
		}
	}
	for _, s := range exc {
		if fn(parseIP(s)) {
			t.Error("returned true for excluded address", s)
		}
	}
}

func parseIP(s string) net.IP {
	ip := net.ParseIP(s)
	if ip == nil {
		panic("invalid " + s)
	}
	return ip
}

func TestCheckRelayIP(t *testing.T) {
	tests := []struct {
		sender, addr string
		want         error
	}{
		{"127.0.0.1", "0.0.0.0", errUnspecified},
		{"192.168.0.1", "0.0.0.0", errUnspecified},
		{"23.55.1.242", "0.0.0.0", errUnspecified},
		{"127.0.0.1", "255.255.255.255", errSpecial},
		{"192.168.0.1", "255.255.255.255", errSpecial},
		{"23.55.1.242", "255.255.255.255", errSpecial},
		{"192.168.0.1", "127.0.2.19", errLoopback},
		{"23.55.1.242", "192.168.0.1", errLAN},

		{"127.0.0.1", "127.0.2.19", nil},
		{"127.0.0.1", "192.168.0.1", nil},
		{"127.0.0.1", "23.55.1.242", nil},
		{"192.168.0.1", "192.168.0.1", nil},
		{"192.168.0.1", "23.55.1.242", nil},
		{"23.55.1.242", "23.55.1.242", nil},
	}

	for _, test := range tests {
		err := CheckRelayIP(parseIP(test.sender), parseIP(test.addr))
		if err != test.want {
			t.Errorf("%s from %s: got %q, want %q", test.addr, test.sender, err, test.want)
		}
	}
}

func BenchmarkCheckRelayIP(b *testing.B) {
	sender := parseIP("23.55.1.242")
	addr := parseIP("23.55.1.2")
	for i := 0; i < b.N; i++ {
		CheckRelayIP(sender, addr)
	}
}

func TestSameNet(t *testing.T) {
	tests := []struct {
		ip, other string
		bits      uint
		want      bool
	}{
		{"0.0.0.0", "0.0.0.0", 32, true},
		{"0.0.0.0", "0.0.0.1", 0, true},
		{"0.0.0.0", "0.0.0.1", 31, true},
		{"0.0.0.0", "0.0.0.1", 32, false},
		{"0.33.0.1", "0.34.0.2", 8, true},
		{"0.33.0.1", "0.34.0.2", 13, true},
		{"0.33.0.1", "0.34.0.2", 15, false},
	}

	for _, test := range tests {
		if ok := SameNet(test.bits, parseIP(test.ip), parseIP(test.other)); ok != test.want {
			t.Errorf("SameNet(%d, %s, %s) == %t, want %t", test.bits, test.ip, test.other, ok, test.want)
		}
	}
}

func ExampleSameNet() {
//这将返回true，因为IP在相同的/24网络中：
	fmt.Println(SameNet(24, net.IP{127, 0, 0, 1}, net.IP{127, 0, 0, 3}))
//此调用返回错误：
	fmt.Println(SameNet(24, net.IP{127, 3, 0, 1}, net.IP{127, 5, 0, 3}))
//输出：
//真
//假
}

func TestDistinctNetSet(t *testing.T) {
	ops := []struct {
		add, remove string
		fails       bool
	}{
		{add: "127.0.0.1"},
		{add: "127.0.0.2"},
		{add: "127.0.0.3", fails: true},
		{add: "127.32.0.1"},
		{add: "127.32.0.2"},
		{add: "127.32.0.3", fails: true},
		{add: "127.33.0.1", fails: true},
		{add: "127.34.0.1"},
		{add: "127.34.0.2"},
		{add: "127.34.0.3", fails: true},
//为地址腾出空间，然后重新添加。
		{remove: "127.0.0.1"},
		{add: "127.0.0.3"},
		{add: "127.0.0.3", fails: true},
	}

	set := DistinctNetSet{Subnet: 15, Limit: 2}
	for _, op := range ops {
		var desc string
		if op.add != "" {
			desc = fmt.Sprintf("Add(%s)", op.add)
			if ok := set.Add(parseIP(op.add)); ok != !op.fails {
				t.Errorf("%s == %t, want %t", desc, ok, !op.fails)
			}
		} else {
			desc = fmt.Sprintf("Remove(%s)", op.remove)
			set.Remove(parseIP(op.remove))
		}
		t.Logf("%s: %v", desc, set)
	}
}

func TestDistinctNetSetAddRemove(t *testing.T) {
	cfg := &quick.Config{}
	fn := func(ips []net.IP) bool {
		s := DistinctNetSet{Limit: 3, Subnet: 2}
		for _, ip := range ips {
			s.Add(ip)
		}
		for _, ip := range ips {
			s.Remove(ip)
		}
		return s.Len() == 0
	}

	if err := quick.Check(fn, cfg); err != nil {
		t.Fatal(err)
	}
}
