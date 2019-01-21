
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package storage

//我们要使用的DB模式。实际/当前数据库架构可能不同
//直到运行迁移。
const CurrentDbSchema = DbSchemaHalloween

//曾经有一段时间我们根本没有模式。
const DbSchemaNone = ""

//“纯度”是我们与Swarm 0.3.5一起发布的第一个级别数据库的正式模式。
const DbSchemaPurity = "purity"

//“万圣节”在这里是因为我们有一个螺丝钉在垃圾回收索引。
//因此，我们必须重建gc索引以消除错误的
//这需要很长的时间。此模式用于记账，
//所以重建索引只运行一次。
const DbSchemaHalloween = "halloween"
