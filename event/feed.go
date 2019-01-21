
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
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

package event

import (
	"errors"
	"reflect"
	"sync"
)

var errBadChannel = errors.New("event: Subscribe argument does not have sendable channel type")

//feed实现了一对多的订阅，其中事件的载体是一个通道。
//发送到提要的值将同时传递到所有订阅的通道。
//
//源只能与单个类型一起使用。类型由第一次发送或
//订阅操作。如果类型不为
//比赛。
//
//零值已准备好使用。
type Feed struct {
once      sync.Once        //确保init只运行一次
sendLock  chan struct{}    //sendlock有一个单元素缓冲区，保持时为空。它保护发送案例。
removeSub chan interface{} //中断发送
sendCases caseList         //the active set of select cases used by Send

//收件箱保存新订阅的频道，直到它们被添加到发送案例中。
	mu     sync.Mutex
	inbox  caseList
	etype  reflect.Type
	closed bool
}

//这是发送案例中第一个实际订阅通道的索引。
//sendCases[0] is a SelectRecv case for the removeSub channel.
const firstSubSendCase = 1

type feedTypeError struct {
	got, want reflect.Type
	op        string
}

func (e feedTypeError) Error() string {
	return "event: wrong type in " + e.op + " got " + e.got.String() + ", want " + e.want.String()
}

func (f *Feed) init() {
	f.removeSub = make(chan interface{})
	f.sendLock = make(chan struct{}, 1)
	f.sendLock <- struct{}{}
	f.sendCases = caseList{{Chan: reflect.ValueOf(f.removeSub), Dir: reflect.SelectRecv}}
}

//订阅向订阅源添加一个频道。未来的发送将在通道上传递
//直到取消订阅。添加的所有通道必须具有相同的元素类型。
//
//通道应该有足够的缓冲空间，以避免阻塞其他订户。
//不会删除速度较慢的订阅服务器。
func (f *Feed) Subscribe(channel interface{}) Subscription {
	f.once.Do(f.init)

	chanval := reflect.ValueOf(channel)
	chantyp := chanval.Type()
	if chantyp.Kind() != reflect.Chan || chantyp.ChanDir()&reflect.SendDir == 0 {
		panic(errBadChannel)
	}
	sub := &feedSub{feed: f, channel: chanval, err: make(chan error, 1)}

	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.typecheck(chantyp.Elem()) {
		panic(feedTypeError{op: "Subscribe", got: chantyp, want: reflect.ChanOf(reflect.SendDir, f.etype)})
	}
//Add the select case to the inbox.
//下一次发送将把它添加到f.sendcases。
	cas := reflect.SelectCase{Dir: reflect.SelectSend, Chan: chanval}
	f.inbox = append(f.inbox, cas)
	return sub
}

//注：呼叫者必须持有F.MU
func (f *Feed) typecheck(typ reflect.Type) bool {
	if f.etype == nil {
		f.etype = typ
		return true
	}
	return f.etype == typ
}

func (f *Feed) remove(sub *feedSub) {
//Delete from inbox first, which covers channels
//尚未添加到f.sendcases。
	ch := sub.channel.Interface()
	f.mu.Lock()
	index := f.inbox.find(ch)
	if index != -1 {
		f.inbox = f.inbox.delete(index)
		f.mu.Unlock()
		return
	}
	f.mu.Unlock()

	select {
	case f.removeSub <- ch:
//send将从f.sendscases中删除通道。
	case <-f.sendLock:
//No Send is in progress, delete the channel now that we have the send lock.
		f.sendCases = f.sendCases.delete(f.sendCases.find(ch))
		f.sendLock <- struct{}{}
	}
}

//发送同时发送到所有订阅的频道。
//它返回值发送到的订户数。
func (f *Feed) Send(value interface{}) (nsent int) {
	rvalue := reflect.ValueOf(value)

	f.once.Do(f.init)
	<-f.sendLock

//获取发送锁定后从收件箱添加新案例。
	f.mu.Lock()
	f.sendCases = append(f.sendCases, f.inbox...)
	f.inbox = nil

	if !f.typecheck(rvalue.Type()) {
		f.sendLock <- struct{}{}
		panic(feedTypeError{op: "Send", got: rvalue.Type(), want: f.etype})
	}
	f.mu.Unlock()

//在所有通道上设置发送值。
	for i := firstSubSendCase; i < len(f.sendCases); i++ {
		f.sendCases[i].Send = rvalue
	}

//发送，直到选择了除removesub以外的所有频道。cases'跟踪前缀
//病例报告。当发送成功时，相应的事例将移动到
//“cases”和它收缩一个元素。
	cases := f.sendCases
	for {
//快速路径：在添加到选择集之前，尝试不阻塞地发送。
//如果订户速度足够快并且有免费服务，这通常会成功。
//缓冲空间。
		for i := firstSubSendCase; i < len(cases); i++ {
			if cases[i].Chan.TrySend(rvalue) {
				nsent++
				cases = cases.deactivate(i)
				i--
			}
		}
		if len(cases) == firstSubSendCase {
			break
		}
//选择所有接收器，等待其解锁。
		chosen, recv, _ := reflect.Select(cases)
  /*已选择==0/*<-f.removesub*/
   索引：=f.sendcases.find（recv.interface（））
   F.sEdvase= f.sEndeStask.删除（索引）
   如果index>=0&&index<len（cases）
    // Shrink 'cases' too because the removed case was still active.
    cases=f.sendcases[：len（cases）-1]
   }
  }否则{
   cases = cases.deactivate(chosen)
   nt++
  }
 }

 //忽略发送值，并关闭发送锁。
 对于i：=firstsubsendcase；i<len（f.sendcases）；i++
  f.sendcases[i].send=reflect.value
 }
 f.sendlock<-结构
 返回N发送
}

类型FEEDSUB结构
 进料*进料
 通道反射值
 误差同步一次
 错误通道错误
}

func（sub*feedsub）unsubscribe（）
 sub.errOnce.Do(func() {
  sub.feed.移除（sub）
  关闭（子）
 }）
}

func（sub*feedsub）err（）<-chan错误
 返回错误
}

键入caselist[]reflect.selectcase

//find返回包含给定通道的事例的索引。
func（cs caselist）find（通道接口）int
 对于I，CAS：=范围CS {
  如果Cas.Chan.IdFACEL（）=通道{
   返回我
  }
 }
 返回- 1
}

//删除从cs中删除给定的case。
func（cs caselist）删除（index int）caselist_
 返回附加（cs[：index]，cs[index+1:…）
}

//deactivate将索引处的大小写移动到cs切片的不可访问部分。
func（cs caselist）deactivate（index int）caselist_
 最后：=len（cs）-1
 cs[索引]，cs[最后]=cs[最后]，cs[索引]
 返回cs[：last]
}

//func（cs caselist）string（）字符串
//s:=“”
//对于i，cas：=范围cs
//如果我！= 0 {
//s+=“，”
//
//切换cas.dir
//case reflect.selectsend：
//s+=fmt.sprintf（“%v<-”，cas.chan.interface（））
//case reflect.selectrecv:
//s+=fmt.sprintf（“<-%v”，cas.chan.interface（））
//
//}
//返回s+“]”
//}
