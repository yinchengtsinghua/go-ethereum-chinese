
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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

/*
包查找定义源查找算法并提供用于放置更新的工具
所以可以找到它们
**/

package lookup

const maxuint64 = ^uint64(0)

//LowestLevel将查找算法的频率分辨率设置为2的幂。
const LowestLevel uint8 = 0 //默认值为0（1秒）

//最高级别设置算法将以2的功率运行的最低频率。
//25->2^25大约等于一年。
const HighestLevel = 25 //默认值为25（~1年）

//默认级别设置没有提示时将选择搜索的级别
const DefaultLevel = HighestLevel

//算法是查找算法的函数签名
type Algorithm func(now uint64, hint Epoch, read ReadFunc) (value interface{}, err error)

//查找查找具有小于或等于“now”的最高时间戳的更新
//它接受了一个提示，应该是最后一次已知更新所在的时代。
//如果您不知道最后一次更新发生在哪个时代，只需提交lookup.noclue
//每次查找时都将调用read（）。
//仅当read（）返回错误时才返回错误
//如果未找到更新，则返回nil
var Lookup Algorithm = FluzCapacitorAlgorithm

//readfunc是lookup在每次试图查找值时调用的处理程序。
//如果找不到值，它应该返回<nil>
//如果找到一个值，它应该返回<nil>，但它的时间戳高于“now”
//它只应在处理程序希望停止时返回错误。
//完全查找过程。
type ReadFunc func(epoch Epoch, now uint64) (interface{}, error)

//noclue是一个提示，当查找调用程序没有
//最后一次更新可能在哪里的线索
var NoClue = Epoch{}

//GetBaseTime返回给定
//时间和水平
func getBaseTime(t uint64, level uint8) uint64 {
	return t & (maxuint64 << level)
}

//提示仅基于上一次已知更新时间创建提示
func Hint(last uint64) Epoch {
	return Epoch{
		Time:  last,
		Level: DefaultLevel,
	}
}

//GetNextLevel返回下一次更新应处于的频率级别，前提是
//上次更新是什么时间。
//这是“last”和“now”的异或的第一个非零位，从最高有效位开始计数。
//但仅限于不返回小于最后一个-1的级别
func GetNextLevel(last Epoch, now uint64) uint8 {
//第一个xor是当前时钟的最后一个epoch基时间。
//这将把所有常见的最高有效位设置为零。
	mix := (last.Base() ^ now)

//然后，通过设置
//这个水平是1。
//如果下一个级别低于当前级别，则必须正好是级别1，而不是更低。
	mix |= (1 << (last.Level - 1))

//如果上一次更新在2^highest level秒之前，请选择最高级别
	if mix > (maxuint64 >> (64 - HighestLevel - 1)) {
		return HighestLevel
	}

//设置一个扫描非零位的掩码，从最高级别开始
	mask := uint64(1 << (HighestLevel))

	for i := uint8(HighestLevel); i > LowestLevel; i-- {
if mix&mask != 0 { //如果我们找到一个非零位，这就是下一次更新应该达到的级别。
			return i
		}
mask = mask >> 1 //把我们的钻头右移一个位置
	}
	return 0
}

//getnextepoch返回下一个更新应位于的epoch
//根据上次更新的位置
//现在几点了。
func GetNextEpoch(last Epoch, now uint64) Epoch {
	if last == NoClue {
		return GetFirstEpoch(now)
	}
	level := GetNextLevel(last, now)
	return Epoch{
		Level: level,
		Time:  now,
	}
}

//GetFirstEpoch返回第一次更新应位于的epoch
//根据现在的时间。
func GetFirstEpoch(now uint64) Epoch {
	return Epoch{Level: HighestLevel, Time: now}
}

var worstHint = Epoch{Time: 0, Level: 63}

//FluzCapacitorAlgorithm的工作原理是，如果找到更新，则缩小epoch搜索区域。
//及时往返
//首先，如果提示是
//最后一次更新。如果查找失败，则最后一次更新必须是提示本身
//或者下面的时代。但是，如果查找成功，则更新必须是
//或者在下面的时代里。
//有关更图形化的表示，请参阅指南。
func FluzCapacitorAlgorithm(now uint64, hint Epoch, read ReadFunc) (value interface{}, err error) {
	var lastFound interface{}
	var epoch Epoch
	if hint == NoClue {
		hint = worstHint
	}

	t := now

	for {
		epoch = GetNextEpoch(hint, t)
		value, err = read(epoch, now)
		if err != nil {
			return nil, err
		}
		if value != nil {
			lastFound = value
			if epoch.Level == LowestLevel || epoch.Equals(hint) {
				return value, nil
			}
			hint = epoch
			continue
		}
		if epoch.Base() == hint.Base() {
			if lastFound != nil {
				return lastFound, nil
			}
//我们自己已经得到了暗示
			if hint == worstHint {
				return nil, nil
			}
//过来看
			value, err = read(hint, now)
			if err != nil {
				return nil, err
			}
			if value != nil {
				return value, nil
			}
//坏提示。
			epoch = hint
			hint = worstHint
		}
		base := epoch.Base()
		if base == 0 {
			return nil, nil
		}
		t = base - 1
	}
}
