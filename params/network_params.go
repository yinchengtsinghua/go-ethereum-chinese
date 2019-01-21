
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

package params

//这些是需要在客户端之间保持不变的网络参数，但是
//不一定与共识有关。

const (
//BloomBitsBlocks是单个BloomBit部分向量的块数。
//包含在服务器端。
	BloomBitsBlocks uint64 = 4096

//BloomBitsBlocksClient是单个BloomBit部分向量的块数。
//在轻型客户端包含
	BloomBitsBlocksClient uint64 = 32768

//BloomConfirms是在Bloom部分
//考虑可能是最终的，并计算其旋转位。
	BloomConfirms = 256

//chtfrequenceclient是在客户端创建cht的块频率。
	CHTFrequencyClient = 32768

//chtfrequencyserver是在服务器端创建cht的块频率。
//最终，这可以与客户端版本合并，但这需要
//完整的数据库升级，所以应该留一段合适的时间。
	CHTFrequencyServer = 4096

//BloomTrieFrequency是在两个对象上创建BloomTrie的块频率。
//服务器/客户端。
	BloomTrieFrequency = 32768

//HelperTrieConfirmations是预期客户端之前的确认数
//提供所给的帮助者。
	HelperTrieConfirmations = 2048

//HelperTrieProcessConfirmations是HelperTrie之前的确认数
//生成
	HelperTrieProcessConfirmations = 256
)
