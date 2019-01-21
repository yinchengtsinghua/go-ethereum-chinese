
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

package ethash

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

var errEthashStopped = errors.New("ethash stopped")

//API为RPC接口公开与ethash相关的方法。
type API struct {
ethash *Ethash //确保ethash模式正常。
}

//GetWork返回外部矿工的工作包。
//
//工作包由3个字符串组成：
//结果[0]-32字节十六进制编码的当前块头POW哈希
//结果[1]-用于DAG的32字节十六进制编码种子哈希
//结果[2]-32字节十六进制编码边界条件（“目标”），2^256/难度
//结果[3]-十六进制编码的块号
func (api *API) GetWork() ([4]string, error) {
	if api.ethash.config.PowMode != ModeNormal && api.ethash.config.PowMode != ModeTest {
		return [4]string{}, errors.New("not supported")
	}

	var (
		workCh = make(chan [4]string, 1)
		errc   = make(chan error, 1)
	)

	select {
	case api.ethash.fetchWorkCh <- &sealWork{errc: errc, res: workCh}:
	case <-api.ethash.exitCh:
		return [4]string{}, errEthashStopped
	}

	select {
	case work := <-workCh:
		return work, nil
	case err := <-errc:
		return [4]string{}, err
	}
}

//外部矿工可使用SubNetwork提交其POW解决方案。
//如果工作被接受，它会返回一个指示。
//注意：如果解决方案无效，则过时的工作不存在的工作将返回false。
func (api *API) SubmitWork(nonce types.BlockNonce, hash, digest common.Hash) bool {
	if api.ethash.config.PowMode != ModeNormal && api.ethash.config.PowMode != ModeTest {
		return false
	}

	var errc = make(chan error, 1)

	select {
	case api.ethash.submitWorkCh <- &mineResult{
		nonce:     nonce,
		mixDigest: digest,
		hash:      hash,
		errc:      errc,
	}:
	case <-api.ethash.exitCh:
		return false
	}

	err := <-errc
	return err == nil
}

//submit hash rate可用于远程矿工提交哈希率。
//这使节点能够报告所有矿工的组合哈希率
//通过这个节点提交工作。
//
//它接受矿工哈希率和标识符，该标识符必须是唯一的
//节点之间。
func (api *API) SubmitHashRate(rate hexutil.Uint64, id common.Hash) bool {
	if api.ethash.config.PowMode != ModeNormal && api.ethash.config.PowMode != ModeTest {
		return false
	}

	var done = make(chan struct{}, 1)

	select {
	case api.ethash.submitRateCh <- &hashrate{done: done, rate: uint64(rate), id: id}:
	case <-api.ethash.exitCh:
		return false
	}

//阻止，直到哈希率成功提交。
	<-done

	return true
}

//GetHashrate返回本地CPU矿工和远程矿工的当前哈希率。
func (api *API) GetHashrate() uint64 {
	return uint64(api.ethash.Hashrate())
}
