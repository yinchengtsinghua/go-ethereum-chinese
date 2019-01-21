
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

package metrics

import (
	"time"

	"github.com/ethereum/go-ethereum/cmd/utils"
	gethmetrics "github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/metrics/influxdb"
	"github.com/ethereum/go-ethereum/swarm/log"
	"gopkg.in/urfave/cli.v1"
)

var (
	MetricsEnableInfluxDBExportFlag = cli.BoolFlag{
		Name:  "metrics.influxdb.export",
		Usage: "Enable metrics export/push to an external InfluxDB database",
	}
	MetricsInfluxDBEndpointFlag = cli.StringFlag{
		Name:  "metrics.influxdb.endpoint",
		Usage: "Metrics InfluxDB endpoint",
Value: "http://127.0.0.1:8086“，
	}
	MetricsInfluxDBDatabaseFlag = cli.StringFlag{
		Name:  "metrics.influxdb.database",
		Usage: "Metrics InfluxDB database",
		Value: "metrics",
	}
	MetricsInfluxDBUsernameFlag = cli.StringFlag{
		Name:  "metrics.influxdb.username",
		Usage: "Metrics InfluxDB username",
		Value: "",
	}
	MetricsInfluxDBPasswordFlag = cli.StringFlag{
		Name:  "metrics.influxdb.password",
		Usage: "Metrics InfluxDB password",
		Value: "",
	}
//“host”标记是发送到influxdb的每个度量的一部分。在influxdb中，对标签的查询更快。
//它的使用是为了让我们可以对所有节点进行分组，并对所有节点的测量值进行平均，但也同样如此
//我们可以选择一个特定的节点并检查它的测量值。
//https://docs.influxdata.com/influxdb/v1.4/concepts/key_-concepts/标记密钥
	MetricsInfluxDBHostTagFlag = cli.StringFlag{
		Name:  "metrics.influxdb.host.tag",
		Usage: "Metrics InfluxDB `host` tag attached to all measurements",
		Value: "localhost",
	}
)

//标志保存度量集合所需的所有命令行标志。
var Flags = []cli.Flag{
	utils.MetricsEnabledFlag,
	MetricsEnableInfluxDBExportFlag,
	MetricsInfluxDBEndpointFlag,
	MetricsInfluxDBDatabaseFlag,
	MetricsInfluxDBUsernameFlag,
	MetricsInfluxDBPasswordFlag,
	MetricsInfluxDBHostTagFlag,
}

func Setup(ctx *cli.Context) {
	if gethmetrics.Enabled {
		log.Info("Enabling swarm metrics collection")
		var (
			enableExport = ctx.GlobalBool(MetricsEnableInfluxDBExportFlag.Name)
			endpoint     = ctx.GlobalString(MetricsInfluxDBEndpointFlag.Name)
			database     = ctx.GlobalString(MetricsInfluxDBDatabaseFlag.Name)
			username     = ctx.GlobalString(MetricsInfluxDBUsernameFlag.Name)
			password     = ctx.GlobalString(MetricsInfluxDBPasswordFlag.Name)
			hosttag      = ctx.GlobalString(MetricsInfluxDBHostTagFlag.Name)
		)

//启动系统运行时度量集合
		go gethmetrics.CollectProcessMetrics(2 * time.Second)

		if enableExport {
			log.Info("Enabling swarm metrics export to InfluxDB")
			go influxdb.InfluxDBWithTags(gethmetrics.DefaultRegistry, 10*time.Second, endpoint, database, username, password, "swarm.", map[string]string{
				"host": hosttag,
			})
		}
	}
}
