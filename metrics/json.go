
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package metrics

import (
	"encoding/json"
	"io"
	"time"
)

//marshaljson返回一个字节片，其中包含所有
//注册表中的指标。
func (r *StandardRegistry) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.GetAll())
}

//WRITEJSON定期将给定注册表中的度量值写入
//指定IO.Writer为JSON。
func WriteJSON(r Registry, d time.Duration, w io.Writer) {
	for range time.Tick(d) {
		WriteJSONOnce(r, w)
	}
}

//writejsonce将给定注册表中的度量值写入指定的
//写JSON。
func WriteJSONOnce(r Registry, w io.Writer) {
	json.NewEncoder(w).Encode(r)
}

func (p *PrefixedRegistry) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.GetAll())
}
