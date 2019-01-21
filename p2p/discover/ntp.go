
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

//包含通过sntp协议进行的ntp时间漂移检测：
//https://tools.ietf.org/html/rfc4330

package discover

import (
	"fmt"
	"net"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

const (
ntpPool   = "pool.ntp.org" //ntppool是要查询当前时间的ntp服务器
ntpChecks = 3              //要对NTP服务器执行的测量数
)

//DurationSicle将Sort.Interface方法附加到[]Time.Duration，
//按递增顺序排序。
type durationSlice []time.Duration

func (s durationSlice) Len() int           { return len(s) }
func (s durationSlice) Less(i, j int) bool { return s[i] < s[j] }
func (s durationSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

//checkclockfloft查询NTP服务器的时钟偏移，并警告用户
//检测到一个足够大的。
func checkClockDrift() {
	drift, err := sntpDrift(ntpChecks)
	if err != nil {
		return
	}
	if drift < -driftThreshold || drift > driftThreshold {
		log.Warn(fmt.Sprintf("System clock seems off by %v, which can prevent network connectivity", drift))
		log.Warn("Please enable network time synchronisation in system settings.")
	} else {
		log.Debug("NTP sanity check done", "drift", drift)
	}
}

//sntpdrift针对ntp服务器执行幼稚的时间解析，并返回
//测量漂移。此方法使用简单版本的NTP。不太准确
//但就这些目的而言应该是好的。
//
//注意，与请求的数量相比，它执行两个额外的测量
//一种能够将这两个极端作为离群值丢弃的方法。
func sntpDrift(measurements int) (time.Duration, error) {
//解析NTP服务器的地址
	addr, err := net.ResolveUDPAddr("udp", ntpPool+":123")
	if err != nil {
		return 0, err
	}
//构造时间请求（仅设置两个字段的空包）：
//位3-5：协议版本，3
//位6-8：操作模式，客户机，3
	request := make([]byte, 48)
	request[0] = 3<<3 | 3

//执行每个测量
	drifts := []time.Duration{}
	for i := 0; i < measurements+2; i++ {
//拨号NTP服务器并发送时间检索请求
		conn, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			return 0, err
		}
		defer conn.Close()

		sent := time.Now()
		if _, err = conn.Write(request); err != nil {
			return 0, err
		}
//检索回复并计算经过的时间
		conn.SetDeadline(time.Now().Add(5 * time.Second))

		reply := make([]byte, 48)
		if _, err = conn.Read(reply); err != nil {
			return 0, err
		}
		elapsed := time.Since(sent)

//从回复数据中重建时间
		sec := uint64(reply[43]) | uint64(reply[42])<<8 | uint64(reply[41])<<16 | uint64(reply[40])<<24
		frac := uint64(reply[47]) | uint64(reply[46])<<8 | uint64(reply[45])<<16 | uint64(reply[44])<<24

		nanosec := sec*1e9 + (frac*1e9)>>32

		t := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(nanosec)).Local()

//根据假定的响应时间rrt/2计算漂移
		drifts = append(drifts, sent.Sub(t)+elapsed/2)
	}
//计算平均干重（减少两端以避免异常值）
	sort.Sort(durationSlice(drifts))

	drift := time.Duration(0)
	for i := 1; i < len(drifts)-1; i++ {
		drift += drifts[i]
	}
	return drift / time.Duration(measurements), nil
}
