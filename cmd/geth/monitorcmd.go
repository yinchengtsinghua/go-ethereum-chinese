
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2015 Go Ethereum作者
//此文件是Go以太坊的一部分。
//
//Go以太坊是免费软件：您可以重新发布和/或修改它
//根据GNU通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊的分布希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU通用公共许可证了解更多详细信息。
//
//你应该已经收到一份GNU通用公共许可证的副本
//一起去以太坊吧。如果没有，请参见<http://www.gnu.org/licenses/>。

package main

import (
	"fmt"
	"math"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gizak/termui"
	"gopkg.in/urfave/cli.v1"
)

var (
	monitorCommandAttachFlag = cli.StringFlag{
		Name:  "attach",
		Value: node.DefaultIPCEndpoint(clientIdentifier),
		Usage: "API endpoint to attach to",
	}
	monitorCommandRowsFlag = cli.IntFlag{
		Name:  "rows",
		Value: 5,
		Usage: "Maximum rows in the chart grid",
	}
	monitorCommandRefreshFlag = cli.IntFlag{
		Name:  "refresh",
		Value: 3,
		Usage: "Refresh interval in seconds",
	}
	monitorCommand = cli.Command{
Action:    utils.MigrateFlags(monitor), //跟踪迁移进度
		Name:      "monitor",
		Usage:     "Monitor and visualize node metrics",
		ArgsUsage: " ",
		Category:  "MONITOR COMMANDS",
		Description: `
The Geth monitor is a tool to collect and visualize various internal metrics
gathered by the node, supporting different chart types as well as the capacity
to display multiple metrics simultaneously.
`,
		Flags: []cli.Flag{
			monitorCommandAttachFlag,
			monitorCommandRowsFlag,
			monitorCommandRefreshFlag,
		},
	}
)

//Monitor为请求的度量启动一个基于终端UI的监视工具。
func monitor(ctx *cli.Context) error {
	var (
		client *rpc.Client
		err    error
	)
//通过IPC或RPC连接到以太坊节点
	endpoint := ctx.String(monitorCommandAttachFlag.Name)
	if client, err = dialRPC(endpoint); err != nil {
		utils.Fatalf("Unable to attach to geth node: %v", err)
	}
	defer client.Close()

//检索所有可用的度量并解析用户模式
	metrics, err := retrieveMetrics(client)
	if err != nil {
		utils.Fatalf("Failed to retrieve system metrics: %v", err)
	}
	monitored := resolveMetrics(metrics, ctx.Args())
	if len(monitored) == 0 {
		list := expandMetrics(metrics, "")
		sort.Strings(list)

		if len(list) > 0 {
			utils.Fatalf("No metrics specified.\n\nAvailable:\n - %s", strings.Join(list, "\n - "))
		} else {
			utils.Fatalf("No metrics collected by geth (--%s).\n", utils.MetricsEnabledFlag.Name)
		}
	}
	sort.Strings(monitored)
	if cols := len(monitored) / ctx.Int(monitorCommandRowsFlag.Name); cols > 6 {
		utils.Fatalf("Requested metrics (%d) spans more that 6 columns:\n - %s", len(monitored), strings.Join(monitored, "\n - "))
	}
//创建和配置图表用户界面默认值
	if err := termui.Init(); err != nil {
		utils.Fatalf("Unable to initialize terminal UI: %v", err)
	}
	defer termui.Close()

	rows := len(monitored)
	if max := ctx.Int(monitorCommandRowsFlag.Name); rows > max {
		rows = max
	}
	cols := (len(monitored) + rows - 1) / rows
	for i := 0; i < rows; i++ {
		termui.Body.AddRows(termui.NewRow())
	}
//创建每个单独的数据图表
	footer := termui.NewPar("")
	footer.Block.Border = true
	footer.Height = 3

	charts := make([]*termui.LineChart, len(monitored))
	units := make([]int, len(monitored))
	data := make([][]float64, len(monitored))
	for i := 0; i < len(monitored); i++ {
		charts[i] = createChart((termui.TermHeight() - footer.Height) / rows)
		row := termui.Body.Rows[i%rows]
		row.Cols = append(row.Cols, termui.NewCol(12/cols, 0, charts[i]))
	}
	termui.Body.AddRows(termui.NewRow(termui.NewCol(12, 0, footer)))

	refreshCharts(client, monitored, data, units, charts, ctx, footer)
	termui.Body.Align()
	termui.Render(termui.Body)

//监视各种系统事件，并定期刷新图表
	termui.Handle("/sys/kbd/C-c", func(termui.Event) {
		termui.StopLoop()
	})
	termui.Handle("/sys/wnd/resize", func(termui.Event) {
		termui.Body.Width = termui.TermWidth()
		for _, chart := range charts {
			chart.Height = (termui.TermHeight() - footer.Height) / rows
		}
		termui.Body.Align()
		termui.Render(termui.Body)
	})
	go func() {
		tick := time.NewTicker(time.Duration(ctx.Int(monitorCommandRefreshFlag.Name)) * time.Second)
		for range tick.C {
			if refreshCharts(client, monitored, data, units, charts, ctx, footer) {
				termui.Body.Align()
			}
			termui.Render(termui.Body)
		}
	}()
	termui.Loop()
	return nil
}

//RetrieveMetrics联系附加的geth节点并检索整个集合
//收集的系统度量。
func retrieveMetrics(client *rpc.Client) (map[string]interface{}, error) {
	var metrics map[string]interface{}
	err := client.Call(&metrics, "debug_metrics", true)
	return metrics, err
}

//ResolveMetrics获取输入度量模式的列表，并将每个模式解析为一个
//或更规范的度量名称。
func resolveMetrics(metrics map[string]interface{}, patterns []string) []string {
	res := []string{}
	for _, pattern := range patterns {
		res = append(res, resolveMetric(metrics, pattern, "")...)
	}
	return res
}

//ResolveMetrics接受一个输入度量模式，并将其解析为一个
//或更规范的度量名称。
func resolveMetric(metrics map[string]interface{}, pattern string, path string) []string {
	results := []string{}

//如果请求嵌套度量，则可以选择性地重复分支（通过逗号）
	parts := strings.SplitN(pattern, "/", 2)
	if len(parts) > 1 {
		for _, variation := range strings.Split(parts[0], ",") {
			submetrics, ok := metrics[variation].(map[string]interface{})
			if !ok {
				utils.Fatalf("Failed to retrieve system metrics: %s", path+variation)
				return nil
			}
			results = append(results, resolveMetric(submetrics, parts[1], path+variation+"/")...)
		}
		return results
	}
//根据最后一个链接是什么，返回或展开
	for _, variation := range strings.Split(pattern, ",") {
		switch metric := metrics[variation].(type) {
		case float64:
//找到最终度量值，返回为singleton
			results = append(results, path+variation)

		case map[string]interface{}:
			results = append(results, expandMetrics(metric, path+variation+"/")...)

		default:
			utils.Fatalf("Metric pattern resolved to unexpected type: %v", reflect.TypeOf(metric))
			return nil
		}
	}
	return results
}

//ExpandMetrics将整个度量树展开为路径的简单列表。
func expandMetrics(metrics map[string]interface{}, path string) []string {
//遍历所有字段并单独展开
	list := []string{}
	for name, metric := range metrics {
		switch metric := metric.(type) {
		case float64:
//找到最终度量值，追加到列表
			list = append(list, path+name)

		case map[string]interface{}:
//找到度量树，递归展开
			list = append(list, expandMetrics(metric, path+name+"/")...)

		default:
			utils.Fatalf("Metric pattern %s resolved to unexpected type: %v", path+name, reflect.TypeOf(metric))
			return nil
		}
	}
	return list
}

//fetchmetric迭代度量图并检索特定的度量图。
func fetchMetric(metrics map[string]interface{}, metric string) float64 {
	parts := strings.Split(metric, "/")
	for _, part := range parts[:len(parts)-1] {
		var found bool
		metrics, found = metrics[part].(map[string]interface{})
		if !found {
			return 0
		}
	}
	if v, ok := metrics[parts[len(parts)-1]].(float64); ok {
		return v
	}
	return 0
}

//刷新图表检索下一批度量，并插入所有新的
//活动数据集和图表中的值
func refreshCharts(client *rpc.Client, metrics []string, data [][]float64, units []int, charts []*termui.LineChart, ctx *cli.Context, footer *termui.Par) (realign bool) {
	values, err := retrieveMetrics(client)
	for i, metric := range metrics {
		if len(data) < 512 {
			data[i] = append([]float64{fetchMetric(values, metric)}, data[i]...)
		} else {
			data[i] = append([]float64{fetchMetric(values, metric)}, data[i][:len(data[i])-1]...)
		}
		if updateChart(metric, data[i], &units[i], charts[i], err) {
			realign = true
		}
	}
	updateFooter(ctx, err, footer)
	return
}

//updateChart将数据集插入折线图中，适当地缩放
//不显示奇怪的标签，也相应地更新图表标签。
func updateChart(metric string, data []float64, base *int, chart *termui.LineChart, err error) (realign bool) {
	dataUnits := []string{"", "K", "M", "G", "T", "E"}
	timeUnits := []string{"ns", "µs", "ms", "s", "ks", "ms"}
	colors := []termui.Attribute{termui.ColorBlue, termui.ColorCyan, termui.ColorGreen, termui.ColorYellow, termui.ColorRed, termui.ColorRed}

//只提取实际可见的部分数据
	if chart.Width*2 < len(data) {
		data = data[:chart.Width*2]
	}
//查找1K以下的最大值和刻度
	high := 0.0
	if len(data) > 0 {
		high = data[0]
		for _, value := range data[1:] {
			high = math.Max(high, value)
		}
	}
	unit, scale := 0, 1.0
	for high >= 1000 && unit+1 < len(dataUnits) {
		high, unit, scale = high/1000, unit+1, scale*1000
	}
//如果单位改变，重新创建图表（hack设置最大高度…）
	if unit != *base {
		realign, *base, *chart = true, unit, *createChart(chart.Height)
	}
//使用缩放值更新图表的数据点
	if cap(chart.Data) < len(data) {
		chart.Data = make([]float64, len(data))
	}
	chart.Data = chart.Data[:len(data)]
	for i, value := range data {
		chart.Data[i] = value / scale
	}
//用刻度单位更新图表标签
	units := dataUnits
	if strings.Contains(metric, "/Percentiles/") || strings.Contains(metric, "/pauses/") || strings.Contains(metric, "/time/") {
		units = timeUnits
	}
	chart.BorderLabel = metric
	if len(units[unit]) > 0 {
		chart.BorderLabel += " [" + units[unit] + "]"
	}
	chart.LineColor = colors[unit] | termui.AttrBold
	if err != nil {
		chart.LineColor = termui.ColorRed | termui.AttrBold
	}
	return
}

//创建带有默认配置的空折线图。
func createChart(height int) *termui.LineChart {
	chart := termui.NewLineChart()
	if runtime.GOOS == "windows" {
		chart.Mode = "dot"
	}
	chart.DataLabels = []string{""}
	chart.Height = height
	chart.AxesColor = termui.ColorWhite
	chart.PaddingBottom = -2

	chart.BorderLabelFg = chart.BorderFg | termui.AttrBold
	chart.BorderFg = chart.BorderBg

	return chart
}

//updateFooter根据遇到的任何错误更新页脚内容。
func updateFooter(ctx *cli.Context, err error, footer *termui.Par) {
//生成基本页脚
	refresh := time.Duration(ctx.Int(monitorCommandRefreshFlag.Name)) * time.Second
	footer.Text = fmt.Sprintf("Press Ctrl+C to quit. Refresh interval: %v.", refresh)
	footer.TextFgColor = termui.ThemeAttr("par.fg") | termui.AttrBold

//附加任何遇到的错误
	if err != nil {
		footer.Text = fmt.Sprintf("Error: %v.", err)
		footer.TextFgColor = termui.ColorRed | termui.AttrBold
	}
}
