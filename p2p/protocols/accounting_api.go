
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package protocols

import (
	"errors"
)

//会计API文本版本号
const AccountingVersion = "1.0"

var errNoAccountingMetrics = errors.New("accounting metrics not enabled")

//AccountingAPI提供了一个API来访问与帐户相关的信息
type AccountingApi struct {
	metrics *AccountingMetrics
}

//new accountingapi创建新的accountingapi
//m将用于检查会计指标是否启用
func NewAccountingApi(m *AccountingMetrics) *AccountingApi {
	return &AccountingApi{m}
}

//余额返回本地节点余额（贷记单位-借记单位）
func (self *AccountingApi) Balance() (int64, error) {
	if self.metrics == nil {
		return 0, errNoAccountingMetrics
	}
	balance := mBalanceCredit.Count() - mBalanceDebit.Count()
	return balance, nil
}

//BalanceCredit返回本地节点贷记的单位总数
func (self *AccountingApi) BalanceCredit() (int64, error) {
	if self.metrics == nil {
		return 0, errNoAccountingMetrics
	}
	return mBalanceCredit.Count(), nil
}

//BalanceCredit返回本地节点借记的单位总数
func (self *AccountingApi) BalanceDebit() (int64, error) {
	if self.metrics == nil {
		return 0, errNoAccountingMetrics
	}
	return mBalanceDebit.Count(), nil
}

//BytesCredit返回本地节点贷记的字节总数
func (self *AccountingApi) BytesCredit() (int64, error) {
	if self.metrics == nil {
		return 0, errNoAccountingMetrics
	}
	return mBytesCredit.Count(), nil
}

//BalanceCredit返回本地节点借记的字节总数
func (self *AccountingApi) BytesDebit() (int64, error) {
	if self.metrics == nil {
		return 0, errNoAccountingMetrics
	}
	return mBytesDebit.Count(), nil
}

//msgcredit返回本地节点贷记的消息总数
func (self *AccountingApi) MsgCredit() (int64, error) {
	if self.metrics == nil {
		return 0, errNoAccountingMetrics
	}
	return mMsgCredit.Count(), nil
}

//
func (self *AccountingApi) MsgDebit() (int64, error) {
	if self.metrics == nil {
		return 0, errNoAccountingMetrics
	}
	return mMsgDebit.Count(), nil
}

//
func (self *AccountingApi) PeerDrops() (int64, error) {
	if self.metrics == nil {
		return 0, errNoAccountingMetrics
	}
	return mPeerDrops.Count(), nil
}

//
func (self *AccountingApi) SelfDrops() (int64, error) {
	if self.metrics == nil {
		return 0, errNoAccountingMetrics
	}
	return mSelfDrops.Count(), nil
}
