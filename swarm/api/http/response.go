
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

package http

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/swarm/api"
)

var (
	htmlCounter      = metrics.NewRegisteredCounter("api.http.errorpage.html.count", nil)
	jsonCounter      = metrics.NewRegisteredCounter("api.http.errorpage.json.count", nil)
	plaintextCounter = metrics.NewRegisteredCounter("api.http.errorpage.plaintext.count", nil)
)

type ResponseParams struct {
	Msg       template.HTML
	Code      int
	Timestamp string
	template  *template.Template
	Details   template.HTML
}

//当用户请求清单中的资源时，将使用showmultipleechoices，结果是
//结果模棱两可。它返回一个HTML页面，其中包含每个条目的可单击链接
//在符合请求URI模糊性的清单中。
//例如，如果用户请求bzz:/<hash>/read，并且该清单包含条目
//“readme.md”和“readinglist.txt”，将返回一个带有这两个链接的HTML页面。
//这仅在清单没有默认条目时适用
func ShowMultipleChoices(w http.ResponseWriter, r *http.Request, list api.ManifestList) {
	log.Debug("ShowMultipleChoices", "ruid", GetRUID(r.Context()), "uri", GetURI(r.Context()))
	msg := ""
	if list.Entries == nil {
		respondError(w, r, "Could not resolve", http.StatusInternalServerError)
		return
	}
	requestUri := strings.TrimPrefix(r.RequestURI, "/")

	uri, err := api.Parse(requestUri)
	if err != nil {
		respondError(w, r, "Bad Request", http.StatusBadRequest)
	}

	uri.Scheme = "bzz-list"
	msg += fmt.Sprintf("Disambiguation:<br/>Your request may refer to multiple choices.<br/>Click <a class=\"orange\" href='"+"/"+uri.String()+"'>here</a> if your browser does not redirect you within 5 seconds.<script>setTimeout(\"location.href='%s';\",5000);</script><br/>", "/"+uri.String())
	respondTemplate(w, r, "error", msg, http.StatusMultipleChoices)
}

func respondTemplate(w http.ResponseWriter, r *http.Request, templateName, msg string, code int) {
	log.Debug("respondTemplate", "ruid", GetRUID(r.Context()), "uri", GetURI(r.Context()))
	respond(w, r, &ResponseParams{
		Code:      code,
		Msg:       template.HTML(msg),
		Timestamp: time.Now().Format(time.RFC1123),
		template:  TemplatesMap[templateName],
	})
}

func respondError(w http.ResponseWriter, r *http.Request, msg string, code int) {
	log.Info("respondError", "ruid", GetRUID(r.Context()), "uri", GetURI(r.Context()), "code", code)
	respondTemplate(w, r, "error", msg, code)
}

func respond(w http.ResponseWriter, r *http.Request, params *ResponseParams) {
	w.WriteHeader(params.Code)

	if params.Code >= 400 {
		w.Header().Del("Cache-Control")
		w.Header().Del("ETag")
	}

	acceptHeader := r.Header.Get("Accept")
 /*这不能在开关中，因为接受头可以有多个值：“accept:*/*，text/html，application/xhtml+xml，application/xml；q=0.9，*/*；q=0.8”
 if strings.contains（acceptHeader，“application/json”）
  如果错误：=respondjson（w，r，params）；错误！= nIL{
   响应程序错误（w，r，“内部服务器错误”，http.statusInternalServerError）
  }
 else if strings.contains（acceptHeader，“text/html”）
  响应HTML（w，r，params）
 }否则{
  respondPlainText（w，r，params）//返回curl的良好错误
 }
}

func respondhtml（w http.responsewriter，r*http.request，params*responseparams）
 HTM计数器公司（1）
 log.info（“respondhtml”，“ruid”，getruid（r.context（）），“code”，params.code）
 错误：=params.template.execute（w，params）
 如果犯错！= nIL{
  日志错误（err.error（））
 }
}

func respondjson（w http.responsewriter，r*http.request，params*responseparams）错误
 jsoncounter.inc（1）公司
 log.info（“respondjson”，“ruid”，getruid（r.context（）），“code”，params.code）
 w.header（）.set（“内容类型”，“应用程序/json”）。
 返回json.newencoder（w）.encode（params）
}

func respondplaintext（w http.responsewriter，r*http.request，params*responseparams）错误
 明文计数器公司（1）
 log.info（“respondPlainText”，“ruid”，getruid（r.context（）），“code”，params.code）
 w.header（）.set（“内容类型”，“文本/普通”）。
 strToWrite：=“代码：”+fmt.sprintf（“%d”，params.code）+\n”
 strToWrite+=“消息：”+string（params.msg）+\n“
 strToWrite+=“时间戳：”+params.timestamp+“”\n“
 _u，err：=w.write（[]byte（strtowrite））。
 返回错误
}
