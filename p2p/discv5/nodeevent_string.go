
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//由“stringer-type=nodeEvent”生成的代码；不要编辑。

package discv5

import "strconv"

const _nodeEvent_name = "pongTimeoutpingTimeoutneighboursTimeout"

var _nodeEvent_index = [...]uint8{0, 11, 22, 39}

func (i nodeEvent) String() string {
	i -= 264
	if i >= nodeEvent(len(_nodeEvent_index)-1) {
		return "nodeEvent(" + strconv.FormatInt(int64(i+264), 10) + ")"
	}
	return _nodeEvent_name[_nodeEvent_index[i]:_nodeEvent_index[i+1]]
}
