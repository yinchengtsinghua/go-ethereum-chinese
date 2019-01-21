
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package influxdb

import (
	"fmt"
	uurl "net/url"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/influxdata/influxdb/client"
)

type reporter struct {
	reg      metrics.Registry
	interval time.Duration

	url       uurl.URL
	database  string
	username  string
	password  string
	namespace string
	tags      map[string]string

	client *client.Client

	cache map[string]int64
}

//influxdb启动一个influxdb报告程序，该报告程序将在每个d间隔发布来自给定metrics.registry的。
func InfluxDB(r metrics.Registry, d time.Duration, url, database, username, password, namespace string) {
	InfluxDBWithTags(r, d, url, database, username, password, namespace, nil)
}

//influxdb with tags启动一个influxdb报告程序，该报告程序将在每个d间隔使用指定的标记从给定的metrics.registry中发布
func InfluxDBWithTags(r metrics.Registry, d time.Duration, url, database, username, password, namespace string, tags map[string]string) {
	u, err := uurl.Parse(url)
	if err != nil {
		log.Warn("Unable to parse InfluxDB", "url", url, "err", err)
		return
	}

	rep := &reporter{
		reg:       r,
		interval:  d,
		url:       *u,
		database:  database,
		username:  username,
		password:  password,
		namespace: namespace,
		tags:      tags,
		cache:     make(map[string]int64),
	}
	if err := rep.makeClient(); err != nil {
		log.Warn("Unable to make InfluxDB client", "err", err)
		return
	}

	rep.run()
}

//influxdbwithTagsOnce运行一次influxdb报告程序并发布给定的度量值。带有指定标记的注册表
func InfluxDBWithTagsOnce(r metrics.Registry, url, database, username, password, namespace string, tags map[string]string) error {
	u, err := uurl.Parse(url)
	if err != nil {
		return fmt.Errorf("Unable to parse InfluxDB. url: %s, err: %v", url, err)
	}

	rep := &reporter{
		reg:       r,
		url:       *u,
		database:  database,
		username:  username,
		password:  password,
		namespace: namespace,
		tags:      tags,
		cache:     make(map[string]int64),
	}
	if err := rep.makeClient(); err != nil {
		return fmt.Errorf("Unable to make InfluxDB client. err: %v", err)
	}

	if err := rep.send(); err != nil {
		return fmt.Errorf("Unable to send to InfluxDB. err: %v", err)
	}

	return nil
}

func (r *reporter) makeClient() (err error) {
	r.client, err = client.NewClient(client.Config{
		URL:      r.url,
		Username: r.username,
		Password: r.password,
	})

	return
}

func (r *reporter) run() {
	intervalTicker := time.Tick(r.interval)
	pingTicker := time.Tick(time.Second * 5)

	for {
		select {
		case <-intervalTicker:
			if err := r.send(); err != nil {
				log.Warn("Unable to send to InfluxDB", "err", err)
			}
		case <-pingTicker:
			_, _, err := r.client.Ping()
			if err != nil {
				log.Warn("Got error while sending a ping to InfluxDB, trying to recreate client", "err", err)

				if err = r.makeClient(); err != nil {
					log.Warn("Unable to make InfluxDB client", "err", err)
				}
			}
		}
	}
}

func (r *reporter) send() error {
	var pts []client.Point

	r.reg.Each(func(name string, i interface{}) {
		now := time.Now()
		namespace := r.namespace

		switch metric := i.(type) {
		case metrics.Counter:
			v := metric.Count()
			l := r.cache[name]
			pts = append(pts, client.Point{
				Measurement: fmt.Sprintf("%s%s.count", namespace, name),
				Tags:        r.tags,
				Fields: map[string]interface{}{
					"value": v - l,
				},
				Time: now,
			})
			r.cache[name] = v
		case metrics.Gauge:
			ms := metric.Snapshot()
			pts = append(pts, client.Point{
				Measurement: fmt.Sprintf("%s%s.gauge", namespace, name),
				Tags:        r.tags,
				Fields: map[string]interface{}{
					"value": ms.Value(),
				},
				Time: now,
			})
		case metrics.GaugeFloat64:
			ms := metric.Snapshot()
			pts = append(pts, client.Point{
				Measurement: fmt.Sprintf("%s%s.gauge", namespace, name),
				Tags:        r.tags,
				Fields: map[string]interface{}{
					"value": ms.Value(),
				},
				Time: now,
			})
		case metrics.Histogram:
			ms := metric.Snapshot()
			ps := ms.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999, 0.9999})
			pts = append(pts, client.Point{
				Measurement: fmt.Sprintf("%s%s.histogram", namespace, name),
				Tags:        r.tags,
				Fields: map[string]interface{}{
					"count":    ms.Count(),
					"max":      ms.Max(),
					"mean":     ms.Mean(),
					"min":      ms.Min(),
					"stddev":   ms.StdDev(),
					"variance": ms.Variance(),
					"p50":      ps[0],
					"p75":      ps[1],
					"p95":      ps[2],
					"p99":      ps[3],
					"p999":     ps[4],
					"p9999":    ps[5],
				},
				Time: now,
			})
		case metrics.Meter:
			ms := metric.Snapshot()
			pts = append(pts, client.Point{
				Measurement: fmt.Sprintf("%s%s.meter", namespace, name),
				Tags:        r.tags,
				Fields: map[string]interface{}{
					"count": ms.Count(),
					"m1":    ms.Rate1(),
					"m5":    ms.Rate5(),
					"m15":   ms.Rate15(),
					"mean":  ms.RateMean(),
				},
				Time: now,
			})
		case metrics.Timer:
			ms := metric.Snapshot()
			ps := ms.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999, 0.9999})
			pts = append(pts, client.Point{
				Measurement: fmt.Sprintf("%s%s.timer", namespace, name),
				Tags:        r.tags,
				Fields: map[string]interface{}{
					"count":    ms.Count(),
					"max":      ms.Max(),
					"mean":     ms.Mean(),
					"min":      ms.Min(),
					"stddev":   ms.StdDev(),
					"variance": ms.Variance(),
					"p50":      ps[0],
					"p75":      ps[1],
					"p95":      ps[2],
					"p99":      ps[3],
					"p999":     ps[4],
					"p9999":    ps[5],
					"m1":       ms.Rate1(),
					"m5":       ms.Rate5(),
					"m15":      ms.Rate15(),
					"meanrate": ms.RateMean(),
				},
				Time: now,
			})
		case metrics.ResettingTimer:
			t := metric.Snapshot()

			if len(t.Values()) > 0 {
				ps := t.Percentiles([]float64{50, 95, 99})
				val := t.Values()
				pts = append(pts, client.Point{
					Measurement: fmt.Sprintf("%s%s.span", namespace, name),
					Tags:        r.tags,
					Fields: map[string]interface{}{
						"count": len(val),
						"max":   val[len(val)-1],
						"mean":  t.Mean(),
						"min":   val[0],
						"p50":   ps[0],
						"p95":   ps[1],
						"p99":   ps[2],
					},
					Time: now,
				})
			}
		}
	})

	bps := client.BatchPoints{
		Points:   pts,
		Database: r.database,
	}

	_, err := r.client.Write(bps)
	return err
}
