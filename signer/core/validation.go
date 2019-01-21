
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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

package core

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"regexp"

	"github.com/ethereum/go-ethereum/common"
)

//验证包包含对事务的验证检查
//-ABI数据验证
//-事务语义验证
//该包为典型的陷阱提供警告

type Validator struct {
	db *AbiDb
}

func NewValidator(db *AbiDb) *Validator {
	return &Validator{db}
}
func testSelector(selector string, data []byte) (*decodedCallData, error) {
	if selector == "" {
		return nil, fmt.Errorf("selector not found")
	}
	abiData, err := MethodSelectorToAbi(selector)
	if err != nil {
		return nil, err
	}
	info, err := parseCallData(data, string(abiData))
	if err != nil {
		return nil, err
	}
	return info, nil

}

//validateCallData检查是否可以解析ABI数据+方法选择器（如果给定），并且似乎匹配
func (v *Validator) validateCallData(msgs *ValidationMessages, data []byte, methodSelector *string) {
	if len(data) == 0 {
		return
	}
	if len(data) < 4 {
		msgs.warn("Tx contains data which is not valid ABI")
		return
	}
	if arglen := len(data) - 4; arglen%32 != 0 {
		msgs.warn(fmt.Sprintf("Not ABI-encoded data; length should be a multiple of 32 (was %d)", arglen))
	}
	var (
		info *decodedCallData
		err  error
	)
//检查提供的一个
	if methodSelector != nil {
		info, err = testSelector(*methodSelector, data)
		if err != nil {
			msgs.warn(fmt.Sprintf("Tx contains data, but provided ABI signature could not be matched: %v", err))
		} else {
			msgs.info(info.String())
//成功完全匹配。如果还没有添加到数据库（忽略其中的错误）
			v.db.AddSignature(*methodSelector, data[:4])
		}
		return
	}
//检查数据库
	selector, err := v.db.LookupMethodSelector(data[:4])
	if err != nil {
		msgs.warn(fmt.Sprintf("Tx contains data, but the ABI signature could not be found: %v", err))
		return
	}
	info, err = testSelector(selector, data)
	if err != nil {
		msgs.warn(fmt.Sprintf("Tx contains data, but provided ABI signature could not be matched: %v", err))
	} else {
		msgs.info(info.String())
	}
}

//validateMantics检查事务是否“有意义”，并为几个典型场景生成警告
func (v *Validator) validate(msgs *ValidationMessages, txargs *SendTxArgs, methodSelector *string) error {
//防止意外错误地使用“输入”和“数据”
	if txargs.Data != nil && txargs.Input != nil && !bytes.Equal(*txargs.Data, *txargs.Input) {
//这是一个展示台
		return errors.New(`Ambiguous request: both "data" and "input" are set and are not identical`)
	}
	var (
		data []byte
	)
//将数据放在“data”上，不输入“input”
	if txargs.Input != nil {
		txargs.Data = txargs.Input
		txargs.Input = nil
	}
	if txargs.Data != nil {
		data = *txargs.Data
	}

	if txargs.To == nil {
//合同创建应包含足够的数据以部署合同
//由于javascript调用中的一些奇怪之处，一个典型的错误是忽略发送者。
//例如：https://github.com/ethereum/go-ethereum/issues/16106
		if len(data) == 0 {
			if txargs.Value.ToInt().Cmp(big.NewInt(0)) > 0 {
//把乙醚送入黑洞
				return errors.New("Tx will create contract with value but empty code!")
			}
//至少没有提交值
			msgs.crit("Tx will create contract with empty code!")
} else if len(data) < 40 { //任意极限
			msgs.warn(fmt.Sprintf("Tx will will create contract, but payload is suspiciously small (%d b)", len(data)))
		}
//对于合同创建，methodSelector应为零
		if methodSelector != nil {
			msgs.warn("Tx will create contract, but method selector supplied; indicating intent to call a method.")
		}

	} else {
		if !txargs.To.ValidChecksum() {
			msgs.warn("Invalid checksum on to-address")
		}
//正常交易
		if bytes.Equal(txargs.To.Address().Bytes(), common.Address{}.Bytes()) {
//发送到0
			msgs.crit("Tx destination is the zero address!")
		}
//验证CallData
		v.validateCallData(msgs, data, methodSelector)
	}
	return nil
}

//validateTransaction对所提供的事务执行许多检查，并返回警告列表，
//或错误，指示应立即拒绝该事务
func (v *Validator) ValidateTransaction(txArgs *SendTxArgs, methodSelector *string) (*ValidationMessages, error) {
	msgs := &ValidationMessages{}
	return msgs, v.validate(msgs, txArgs, methodSelector)
}

var Printable7BitAscii = regexp.MustCompile("^[A-Za-z0-9!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~ ]+$")

//如果密码太短或包含字符，则validatePasswordFormat返回错误。
//超出可打印7bit ASCII集的范围
func ValidatePasswordFormat(password string) error {
	if len(password) < 10 {
		return errors.New("password too short (<10 characters)")
	}
	if !Printable7BitAscii.MatchString(password) {
		return errors.New("password contains invalid characters - only 7bit printable ascii allowed")
	}
	return nil
}
