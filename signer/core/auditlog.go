
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
	"context"

	"encoding/json"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
)

type AuditLogger struct {
	log log.Logger
	api ExternalAPI
}

func (l *AuditLogger) List(ctx context.Context) ([]common.Address, error) {
	l.log.Info("List", "type", "request", "metadata", MetadataFromContext(ctx).String())
	res, e := l.api.List(ctx)
	l.log.Info("List", "type", "response", "data", res)

	return res, e
}

func (l *AuditLogger) New(ctx context.Context) (accounts.Account, error) {
	return l.api.New(ctx)
}

func (l *AuditLogger) SignTransaction(ctx context.Context, args SendTxArgs, methodSelector *string) (*ethapi.SignTransactionResult, error) {
	sel := "<nil>"
	if methodSelector != nil {
		sel = *methodSelector
	}
	l.log.Info("SignTransaction", "type", "request", "metadata", MetadataFromContext(ctx).String(),
		"tx", args.String(),
		"methodSelector", sel)

	res, e := l.api.SignTransaction(ctx, args, methodSelector)
	if res != nil {
		l.log.Info("SignTransaction", "type", "response", "data", common.Bytes2Hex(res.Raw), "error", e)
	} else {
		l.log.Info("SignTransaction", "type", "response", "data", res, "error", e)
	}
	return res, e
}

func (l *AuditLogger) Sign(ctx context.Context, addr common.MixedcaseAddress, data hexutil.Bytes) (hexutil.Bytes, error) {
	l.log.Info("Sign", "type", "request", "metadata", MetadataFromContext(ctx).String(),
		"addr", addr.String(), "data", common.Bytes2Hex(data))
	b, e := l.api.Sign(ctx, addr, data)
	l.log.Info("Sign", "type", "response", "data", common.Bytes2Hex(b), "error", e)
	return b, e
}

func (l *AuditLogger) Export(ctx context.Context, addr common.Address) (json.RawMessage, error) {
	l.log.Info("Export", "type", "request", "metadata", MetadataFromContext(ctx).String(),
		"addr", addr.Hex())
	j, e := l.api.Export(ctx, addr)
//在这种情况下，我们实际上不记录JSON响应，这可能是非常敏感的。
	l.log.Info("Export", "type", "response", "json response size", len(j), "error", e)
	return j, e
}

//func（l*auditlogger）导入（ctx context.context，keyjson json.rawmessage）（account，error）
////不要记录JSON内容
//l.log.info（“导入”，“类型”，“请求”，“元数据”，metadataFromContext（ctx.string（），
//“keyjson大小”，len（keyjson））
//a，e：=l.api.import（ctx，keyjson）
//l.log.info（“导入”，“类型”，“响应”，“地址”，a.string（），“错误”，e）
//返回A，E
//}

func NewAuditLogger(path string, api ExternalAPI) (*AuditLogger, error) {
	l := log.New("api", "signer")
	handler, err := log.FileHandler(path, log.LogfmtFormat())
	if err != nil {
		return nil, err
	}
	l.SetHandler(handler)
	l.Info("Configured", "audit log", path)
	return &AuditLogger{l, api}, nil
}
