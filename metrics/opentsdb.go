
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package metrics

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

var shortHostName string = ""

//OpenTSDBConfig为容器提供配置参数
//OpenTSDB导出程序
type OpenTSDBConfig struct {
Addr          *net.TCPAddr  //要连接到的网络地址
Registry      Registry      //要导出的注册表
FlushInterval time.Duration //冲洗间隔
DurationUnit  time.Duration //持续时间的时间转换单位
Prefix        string        //要加在指标名称前面的前缀
}

//OpenTSDB是一个阻塞导出函数，它在R中报告度量值
//到位于addr的tsdb服务器，每隔d段时间刷新一次
//并在指标名称前面加前缀。
func OpenTSDB(r Registry, d time.Duration, prefix string, addr *net.TCPAddr) {
	OpenTSDBWithConfig(OpenTSDBConfig{
		Addr:          addr,
		Registry:      r,
		FlushInterval: d,
		DurationUnit:  time.Nanosecond,
		Prefix:        prefix,
	})
}

//OpenTSDBWithConfig是一个阻塞导出函数，就像OpenTSDB一样，
//但是它需要一个OpenTSdbconfig。
func OpenTSDBWithConfig(c OpenTSDBConfig) {
	for range time.Tick(c.FlushInterval) {
		if err := openTSDB(&c); nil != err {
			log.Println(err)
		}
	}
}

func getShortHostname() string {
	if shortHostName == "" {
		host, _ := os.Hostname()
		if index := strings.Index(host, "."); index > 0 {
			shortHostName = host[:index]
		} else {
			shortHostName = host
		}
	}
	return shortHostName
}

func openTSDB(c *OpenTSDBConfig) error {
	shortHostname := getShortHostname()
	now := time.Now().Unix()
	du := float64(c.DurationUnit)
	conn, err := net.DialTCP("tcp", nil, c.Addr)
	if nil != err {
		return err
	}
	defer conn.Close()
	w := bufio.NewWriter(conn)
	c.Registry.Each(func(name string, i interface{}) {
		switch metric := i.(type) {
		case Counter:
			fmt.Fprintf(w, "put %s.%s.count %d %d host=%s\n", c.Prefix, name, now, metric.Count(), shortHostname)
		case Gauge:
			fmt.Fprintf(w, "put %s.%s.value %d %d host=%s\n", c.Prefix, name, now, metric.Value(), shortHostname)
		case GaugeFloat64:
			fmt.Fprintf(w, "put %s.%s.value %d %f host=%s\n", c.Prefix, name, now, metric.Value(), shortHostname)
		case Histogram:
			h := metric.Snapshot()
			ps := h.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999})
			fmt.Fprintf(w, "put %s.%s.count %d %d host=%s\n", c.Prefix, name, now, h.Count(), shortHostname)
			fmt.Fprintf(w, "put %s.%s.min %d %d host=%s\n", c.Prefix, name, now, h.Min(), shortHostname)
			fmt.Fprintf(w, "put %s.%s.max %d %d host=%s\n", c.Prefix, name, now, h.Max(), shortHostname)
			fmt.Fprintf(w, "put %s.%s.mean %d %.2f host=%s\n", c.Prefix, name, now, h.Mean(), shortHostname)
			fmt.Fprintf(w, "put %s.%s.std-dev %d %.2f host=%s\n", c.Prefix, name, now, h.StdDev(), shortHostname)
			fmt.Fprintf(w, "put %s.%s.50-percentile %d %.2f host=%s\n", c.Prefix, name, now, ps[0], shortHostname)
			fmt.Fprintf(w, "put %s.%s.75-percentile %d %.2f host=%s\n", c.Prefix, name, now, ps[1], shortHostname)
			fmt.Fprintf(w, "put %s.%s.95-percentile %d %.2f host=%s\n", c.Prefix, name, now, ps[2], shortHostname)
			fmt.Fprintf(w, "put %s.%s.99-percentile %d %.2f host=%s\n", c.Prefix, name, now, ps[3], shortHostname)
			fmt.Fprintf(w, "put %s.%s.999-percentile %d %.2f host=%s\n", c.Prefix, name, now, ps[4], shortHostname)
		case Meter:
			m := metric.Snapshot()
			fmt.Fprintf(w, "put %s.%s.count %d %d host=%s\n", c.Prefix, name, now, m.Count(), shortHostname)
			fmt.Fprintf(w, "put %s.%s.one-minute %d %.2f host=%s\n", c.Prefix, name, now, m.Rate1(), shortHostname)
			fmt.Fprintf(w, "put %s.%s.five-minute %d %.2f host=%s\n", c.Prefix, name, now, m.Rate5(), shortHostname)
			fmt.Fprintf(w, "put %s.%s.fifteen-minute %d %.2f host=%s\n", c.Prefix, name, now, m.Rate15(), shortHostname)
			fmt.Fprintf(w, "put %s.%s.mean %d %.2f host=%s\n", c.Prefix, name, now, m.RateMean(), shortHostname)
		case Timer:
			t := metric.Snapshot()
			ps := t.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999})
			fmt.Fprintf(w, "put %s.%s.count %d %d host=%s\n", c.Prefix, name, now, t.Count(), shortHostname)
			fmt.Fprintf(w, "put %s.%s.min %d %d host=%s\n", c.Prefix, name, now, t.Min()/int64(du), shortHostname)
			fmt.Fprintf(w, "put %s.%s.max %d %d host=%s\n", c.Prefix, name, now, t.Max()/int64(du), shortHostname)
			fmt.Fprintf(w, "put %s.%s.mean %d %.2f host=%s\n", c.Prefix, name, now, t.Mean()/du, shortHostname)
			fmt.Fprintf(w, "put %s.%s.std-dev %d %.2f host=%s\n", c.Prefix, name, now, t.StdDev()/du, shortHostname)
			fmt.Fprintf(w, "put %s.%s.50-percentile %d %.2f host=%s\n", c.Prefix, name, now, ps[0]/du, shortHostname)
			fmt.Fprintf(w, "put %s.%s.75-percentile %d %.2f host=%s\n", c.Prefix, name, now, ps[1]/du, shortHostname)
			fmt.Fprintf(w, "put %s.%s.95-percentile %d %.2f host=%s\n", c.Prefix, name, now, ps[2]/du, shortHostname)
			fmt.Fprintf(w, "put %s.%s.99-percentile %d %.2f host=%s\n", c.Prefix, name, now, ps[3]/du, shortHostname)
			fmt.Fprintf(w, "put %s.%s.999-percentile %d %.2f host=%s\n", c.Prefix, name, now, ps[4]/du, shortHostname)
			fmt.Fprintf(w, "put %s.%s.one-minute %d %.2f host=%s\n", c.Prefix, name, now, t.Rate1(), shortHostname)
			fmt.Fprintf(w, "put %s.%s.five-minute %d %.2f host=%s\n", c.Prefix, name, now, t.Rate5(), shortHostname)
			fmt.Fprintf(w, "put %s.%s.fifteen-minute %d %.2f host=%s\n", c.Prefix, name, now, t.Rate15(), shortHostname)
			fmt.Fprintf(w, "put %s.%s.mean-rate %d %.2f host=%s\n", c.Prefix, name, now, t.RateMean(), shortHostname)
		}
		w.Flush()
	})
	return nil
}
