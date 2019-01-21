
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package spancontext

import (
	"context"

	opentracing "github.com/opentracing/opentracing-go"
)

func WithContext(ctx context.Context, sctx opentracing.SpanContext) context.Context {
	return context.WithValue(ctx, "span_context", sctx)
}

func FromContext(ctx context.Context) opentracing.SpanContext {
	sctx, ok := ctx.Value("span_context").(opentracing.SpanContext)
	if ok {
		return sctx
	}

	return nil
}

func StartSpan(ctx context.Context, name string) (context.Context, opentracing.Span) {
	tracer := opentracing.GlobalTracer()

	sctx := FromContext(ctx)

	var sp opentracing.Span
	if sctx != nil {
		sp = tracer.StartSpan(
			name,
			opentracing.ChildOf(sctx))
	} else {
		sp = tracer.StartSpan(name)
	}

	nctx := context.WithValue(ctx, "span_context", sp.Context())

	return nctx, sp
}

func StartSpanFrom(name string, sctx opentracing.SpanContext) opentracing.Span {
	tracer := opentracing.GlobalTracer()

	sp := tracer.StartSpan(
		name,
		opentracing.ChildOf(sctx))

	return sp
}
