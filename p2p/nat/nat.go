
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

//包NAT提供对公共网络端口映射协议的访问。
package nat

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/jackpal/go-nat-pmp"
)

//一个nat.interface的实现可以将本地端口映射到端口
//可从Internet访问。
type Interface interface {
//这些方法管理本地端口之间的映射
//机器到可以从Internet连接到的端口。
//
//协议是“udp”或“tcp”。某些实现允许设置
//映射的显示名称。映射可以通过
//网关的生命周期结束时。
	AddMapping(protocol string, extport, intport int, name string, lifetime time.Duration) error
	DeleteMapping(protocol string, extport, intport int) error

//此方法应返回外部（面向Internet）
//网关设备的地址。
	ExternalIP() (net.IP, error)

//应返回方法的名称。这用于日志记录。
	String() string
}

//解析解析一个NAT接口描述。
//当前接受以下格式。
//请注意，机制名称不区分大小写。
//
//“”或“无”返回零
//“extip:77.12.33.4”将假定在给定的IP上可以访问本地计算机。
//“任意”使用第一个自动检测机制
//“UPNP”使用通用即插即用协议
//“pmp”使用具有自动检测到的网关地址的nat-pmp
//“pmp:192.168.0.1”使用具有给定网关地址的nat-pmp
func Parse(spec string) (Interface, error) {
	var (
		parts = strings.SplitN(spec, ":", 2)
		mech  = strings.ToLower(parts[0])
		ip    net.IP
	)
	if len(parts) > 1 {
		ip = net.ParseIP(parts[1])
		if ip == nil {
			return nil, errors.New("invalid IP address")
		}
	}
	switch mech {
	case "", "none", "off":
		return nil, nil
	case "any", "auto", "on":
		return Any(), nil
	case "extip", "ip":
		if ip == nil {
			return nil, errors.New("missing IP address")
		}
		return ExtIP(ip), nil
	case "upnp":
		return UPnP(), nil
	case "pmp", "natpmp", "nat-pmp":
		return PMP(ip), nil
	default:
		return nil, fmt.Errorf("unknown mechanism %q", parts[0])
	}
}

const (
	mapTimeout        = 20 * time.Minute
	mapUpdateInterval = 15 * time.Minute
)

//map在m上添加了一个端口映射，并在c关闭之前保持它的活动状态。
//此函数通常在自己的goroutine中调用。
func Map(m Interface, c chan struct{}, protocol string, extport, intport int, name string) {
	log := log.New("proto", protocol, "extport", extport, "intport", intport, "interface", m)
	refresh := time.NewTimer(mapUpdateInterval)
	defer func() {
		refresh.Stop()
		log.Debug("Deleting port mapping")
		m.DeleteMapping(protocol, extport, intport)
	}()
	if err := m.AddMapping(protocol, extport, intport, name, mapTimeout); err != nil {
		log.Debug("Couldn't add port mapping", "err", err)
	} else {
		log.Info("Mapped network port")
	}
	for {
		select {
		case _, ok := <-c:
			if !ok {
				return
			}
		case <-refresh.C:
			log.Trace("Refreshing port mapping")
			if err := m.AddMapping(protocol, extport, intport, name, mapTimeout); err != nil {
				log.Debug("Couldn't add port mapping", "err", err)
			}
			refresh.Reset(mapUpdateInterval)
		}
	}
}

//extip假定在给定的
//外部IP地址，以及手动映射所需的任何端口。
//映射操作不会返回错误，但实际上不会执行任何操作。
type ExtIP net.IP

func (n ExtIP) ExternalIP() (net.IP, error) { return net.IP(n), nil }
func (n ExtIP) String() string              { return fmt.Sprintf("ExtIP(%v)", net.IP(n)) }

//这些什么都不做。

func (ExtIP) AddMapping(string, int, int, string, time.Duration) error { return nil }
func (ExtIP) DeleteMapping(string, int, int) error                     { return nil }

//any返回一个端口映射器，该映射器尝试发现任何支持的
//本地网络上的机制。
func Any() Interface {
//TODO:尝试发现本地计算机是否具有
//Internet类地址。在这种情况下返回extip。
	return startautodisc("UPnP or NAT-PMP", func() Interface {
		found := make(chan Interface, 2)
		go func() { found <- discoverUPnP() }()
		go func() { found <- discoverPMP() }()
		for i := 0; i < cap(found); i++ {
			if c := <-found; c != nil {
				return c
			}
		}
		return nil
	})
}

//UPNP返回使用UPNP的端口映射器。它将试图
//使用UDP广播发现路由器的地址。
func UPnP() Interface {
	return startautodisc("UPnP", discoverUPnP)
}

//pmp返回使用nat-pmp的端口映射器。提供的网关
//地址应该是路由器的IP。如果给定的网关
//地址为零，PMP将尝试自动发现路由器。
func PMP(gateway net.IP) Interface {
	if gateway != nil {
		return &pmp{gw: gateway, c: natpmp.NewClient(gateway)}
	}
	return startautodisc("NAT-PMP", discoverPMP)
}

//自动发现表示仍在
//自动发现。调用此类型上的接口方法将
//等待发现完成，然后在
//发现了机制。
//
//这种类型很有用，因为发现可能需要一段时间，但是我们
//要立即从upnp、pmp和auto返回接口值。
type autodisc struct {
what string //正在自动发现的接口类型
	once sync.Once
	doit func() Interface

	mu    sync.Mutex
	found Interface
}

func startautodisc(what string, doit func() Interface) Interface {
//TODO:监视网络配置，并在其更改时重新运行DOIT。
	return &autodisc{what: what, doit: doit}
}

func (n *autodisc) AddMapping(protocol string, extport, intport int, name string, lifetime time.Duration) error {
	if err := n.wait(); err != nil {
		return err
	}
	return n.found.AddMapping(protocol, extport, intport, name, lifetime)
}

func (n *autodisc) DeleteMapping(protocol string, extport, intport int) error {
	if err := n.wait(); err != nil {
		return err
	}
	return n.found.DeleteMapping(protocol, extport, intport)
}

func (n *autodisc) ExternalIP() (net.IP, error) {
	if err := n.wait(); err != nil {
		return nil, err
	}
	return n.found.ExternalIP()
}

func (n *autodisc) String() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.found == nil {
		return n.what
	} else {
		return n.found.String()
	}
}

//等待块，直到执行自动发现。
func (n *autodisc) wait() error {
	n.once.Do(func() {
		n.mu.Lock()
		n.found = n.doit()
		n.mu.Unlock()
	})
	if n.found == nil {
		return fmt.Errorf("no %s router discovered", n.what)
	}
	return nil
}
