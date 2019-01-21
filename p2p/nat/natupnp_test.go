
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

package nat

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"testing"

	"github.com/huin/goupnp/httpu"
)

func TestUPNP_DDWRT(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("disabled to avoid firewall prompt")
	}

	dev := &fakeIGD{
		t: t,
		ssdpResp: "HTTP/1.1 200 OK\r\n" +
			"Cache-Control: max-age=300\r\n" +
			"Date: Sun, 10 May 2015 10:05:33 GMT\r\n" +
			"Ext: \r\n" +
"Location: http://listenaddr/internetgatewaydevice.xml\r\n“+
			"Server: POSIX UPnP/1.0 DD-WRT Linux/V24\r\n" +
			"ST: urn:schemas-upnp-org:device:WANConnectionDevice:1\r\n" +
			"USN: uuid:CB2471CC-CF2E-9795-8D9C-E87B34C16800::urn:schemas-upnp-org:device:WANConnectionDevice:1\r\n" +
			"\r\n",
		httpResps: map[string]string{
			"GET /InternetGatewayDevice.xml": `
				 <?xml version="1.0"?>
				 <root xmlns="urn:schemas-upnp-org:device-1-0">
					 <specVersion>
						 <major>1</major>
						 <minor>0</minor>
					 </specVersion>
					 <device>
						 <deviceType>urn:schemas-upnp-org:device:InternetGatewayDevice:1</deviceType>
						 <manufacturer>DD-WRT</manufacturer>
<manufacturerURL>http://www.dd-wrt.com</manufacturerurl>
						 <modelDescription>Gateway</modelDescription>
						 <friendlyName>Asus RT-N16:DD-WRT</friendlyName>
						 <modelName>Asus RT-N16</modelName>
						 <modelNumber>V24</modelNumber>
						 <serialNumber>0000001</serialNumber>
<modelURL>http://www.dd-wrt.com</modelurl>
						 <UDN>uuid:A13AB4C3-3A14-E386-DE6A-EFEA923A06FE</UDN>
						 <serviceList>
							 <service>
								 <serviceType>urn:schemas-upnp-org:service:Layer3Forwarding:1</serviceType>
								 <serviceId>urn:upnp-org:serviceId:L3Forwarding1</serviceId>
								 <SCPDURL>/x_layer3forwarding.xml</SCPDURL>
								 <controlURL>/control?Layer3Forwarding</controlURL>
								 <eventSubURL>/event?Layer3Forwarding</eventSubURL>
							 </service>
						 </serviceList>
						 <deviceList>
							 <device>
								 <deviceType>urn:schemas-upnp-org:device:WANDevice:1</deviceType>
								 <friendlyName>WANDevice</friendlyName>
								 <manufacturer>DD-WRT</manufacturer>
<manufacturerURL>http://www.dd-wrt.com</manufacturerurl>
								 <modelDescription>Gateway</modelDescription>
								 <modelName>router</modelName>
<modelURL>http://www.dd-wrt.com</modelurl>
								 <UDN>uuid:48FD569B-F9A9-96AE-4EE6-EB403D3DB91A</UDN>
								 <serviceList>
									 <service>
										 <serviceType>urn:schemas-upnp-org:service:WANCommonInterfaceConfig:1</serviceType>
										 <serviceId>urn:upnp-org:serviceId:WANCommonIFC1</serviceId>
										 <SCPDURL>/x_wancommoninterfaceconfig.xml</SCPDURL>
										 <controlURL>/control?WANCommonInterfaceConfig</controlURL>
										 <eventSubURL>/event?WANCommonInterfaceConfig</eventSubURL>
									 </service>
								 </serviceList>
								 <deviceList>
									 <device>
										 <deviceType>urn:schemas-upnp-org:device:WANConnectionDevice:1</deviceType>
										 <friendlyName>WAN Connection Device</friendlyName>
										 <manufacturer>DD-WRT</manufacturer>
<manufacturerURL>http://www.dd-wrt.com</manufacturerurl>
										 <modelDescription>Gateway</modelDescription>
										 <modelName>router</modelName>
<modelURL>http://www.dd-wrt.com</modelurl>
										 <UDN>uuid:CB2471CC-CF2E-9795-8D9C-E87B34C16800</UDN>
										 <serviceList>
											 <service>
												 <serviceType>urn:schemas-upnp-org:service:WANIPConnection:1</serviceType>
												 <serviceId>urn:upnp-org:serviceId:WANIPConn1</serviceId>
												 <SCPDURL>/x_wanipconnection.xml</SCPDURL>
												 <controlURL>/control?WANIPConnection</controlURL>
												 <eventSubURL>/event?WANIPConnection</eventSubURL>
											 </service>
										 </serviceList>
									 </device>
								 </deviceList>
							 </device>
							 <device>
								 <deviceType>urn:schemas-upnp-org:device:LANDevice:1</deviceType>
								 <friendlyName>LANDevice</friendlyName>
								 <manufacturer>DD-WRT</manufacturer>
<manufacturerURL>http://www.dd-wrt.com</manufacturerurl>
								 <modelDescription>Gateway</modelDescription>
								 <modelName>router</modelName>
<modelURL>http://www.dd-wrt.com</modelurl>
								 <UDN>uuid:04021998-3B35-2BDB-7B3C-99DA4435DA09</UDN>
								 <serviceList>
									 <service>
										 <serviceType>urn:schemas-upnp-org:service:LANHostConfigManagement:1</serviceType>
										 <serviceId>urn:upnp-org:serviceId:LANHostCfg1</serviceId>
										 <SCPDURL>/x_lanhostconfigmanagement.xml</SCPDURL>
										 <controlURL>/control?LANHostConfigManagement</controlURL>
										 <eventSubURL>/event?LANHostConfigManagement</eventSubURL>
									 </service>
								 </serviceList>
							 </device>
						 </deviceList>
<presentationURL>http://listenaddr<presentationurl>
					 </device>
				 </root>
			`,
//对GetNatrSipstatus调用的响应。这个
//特定的实现有一个bug，其中元素
//U内：GetNatrSipstatusResponse不正确
//命名空间。
			"POST /control?WANIPConnection": `
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/“s:encodingstyle=”http://schemas.xmlsoap.org/soap/encoding/“>
				 <s:Body>
				 <u:GetNATRSIPStatusResponse xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1">
				 <NewRSIPAvailable>0</NewRSIPAvailable>
				 <NewNATEnabled>1</NewNATEnabled>
				 </u:GetNATRSIPStatusResponse>
				 </s:Body>
				 </s:Envelope>
			`,
		},
	}
	if err := dev.listen(); err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	dev.serve()
	defer dev.close()

//尝试发现假设备。
	discovered := discoverUPnP()
	if discovered == nil {
		t.Fatalf("not discovered")
	}
	upnp, _ := discovered.(*upnp)
	if upnp.service != "IGDv1-IP1" {
		t.Errorf("upnp.service mismatch: got %q, want %q", upnp.service, "IGDv1-IP1")
	}
wantURL := "http://“+dev.listener.addr（）.string（）+”/internetgatewaydevice.xml“
	if upnp.dev.URLBaseStr != wantURL {
		t.Errorf("upnp.dev.URLBaseStr mismatch: got %q, want %q", upnp.dev.URLBaseStr, wantURL)
	}
}

//Fakeigd将自己呈现为一个可发现的UPNP设备，它发送
//对httpu和http请求的屏蔽响应。
type fakeIGD struct {
t *testing.T //用于测井

	listener      net.Listener
	mcastListener *net.UDPConn

//这应该是一个完整的HTTP响应（包括头）。
//它作为对任何SSPD包的响应发送。任何情况
//将“listenaddr”的替换为实际的TCP侦听
//HTTP服务器的地址。
	ssdpResp string
//这个应该包含所有请求的XML有效负载
//进行。键包含方法和路径，例如“get/foo/bar”。
//与ssdprep一样，“listenaddr”替换为tcp
//听地址。
	httpResps map[string]string
}

//处理程序
func (dev *fakeIGD) ServeMessage(r *http.Request) {
	dev.t.Logf(`HTTPU request %s %s`, r.Method, r.RequestURI)
	conn, err := net.Dial("udp4", r.RemoteAddr)
	if err != nil {
		fmt.Printf("reply Dial error: %v", err)
		return
	}
	defer conn.Close()
	io.WriteString(conn, dev.replaceListenAddr(dev.ssdpResp))
}

//http处理程序
func (dev *fakeIGD) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if resp, ok := dev.httpResps[r.Method+" "+r.RequestURI]; ok {
		dev.t.Logf(`HTTP request "%s %s" --> %d`, r.Method, r.RequestURI, 200)
		io.WriteString(w, dev.replaceListenAddr(resp))
	} else {
		dev.t.Logf(`HTTP request "%s %s" --> %d`, r.Method, r.RequestURI, 404)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (dev *fakeIGD) replaceListenAddr(resp string) string {
	return strings.Replace(resp, "{{listenAddr}}", dev.listener.Addr().String(), -1)
}

func (dev *fakeIGD) listen() (err error) {
	if dev.listener, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
		return err
	}
	laddr := &net.UDPAddr{IP: net.ParseIP("239.255.255.250"), Port: 1900}
	if dev.mcastListener, err = net.ListenMulticastUDP("udp", nil, laddr); err != nil {
		dev.listener.Close()
		return err
	}
	return nil
}

func (dev *fakeIGD) serve() {
	go httpu.Serve(dev.mcastListener, dev)
	go http.Serve(dev.listener, dev)
}

func (dev *fakeIGD) close() {
	dev.mcastListener.Close()
	dev.listener.Close()
}
