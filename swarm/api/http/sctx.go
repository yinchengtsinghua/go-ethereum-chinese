
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package http

import (
	"context"

	"github.com/ethereum/go-ethereum/swarm/api"
	"github.com/ethereum/go-ethereum/swarm/sctx"
)

type uriKey struct{}

func GetRUID(ctx context.Context) string {
	v, ok := ctx.Value(sctx.HTTPRequestIDKey{}).(string)
	if ok {
		return v
	}
	return "xxxxxxxx"
}

func SetRUID(ctx context.Context, ruid string) context.Context {
	return context.WithValue(ctx, sctx.HTTPRequestIDKey{}, ruid)
}

func GetURI(ctx context.Context) *api.URI {
	v, ok := ctx.Value(uriKey{}).(*api.URI)
	if ok {
		return v
	}
	return nil
}

func SetURI(ctx context.Context, uri *api.URI) context.Context {
	return context.WithValue(ctx, uriKey{}, uri)
}
