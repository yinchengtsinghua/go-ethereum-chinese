
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

package swap

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/chequebook"
	"github.com/ethereum/go-ethereum/contracts/chequebook/contract"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/swarm/log"
	"github.com/ethereum/go-ethereum/swarm/services/swap/swap"
)

//交换Swarm会计协议
//交换^2预扣自动付款策略
//交换^3认证：通过信用交换付款
//使用支票簿包延迟付款
//默认参数

var (
autoCashInterval     = 300 * time.Second           //自动清除的默认间隔
autoCashThreshold    = big.NewInt(50000000000000)  //触发自动清除的阈值（wei）
autoDepositInterval  = 300 * time.Second           //自动清除的默认间隔
autoDepositThreshold = big.NewInt(50000000000000)  //触发自动报告的阈值（wei）
autoDepositBuffer    = big.NewInt(100000000000000) //剩余用于叉保护等的缓冲器（WEI）
buyAt                = big.NewInt(20000000000)     //主机愿意支付的最高价（WEI）
sellAt               = big.NewInt(20000000000)     //主机要求的最小批量价格（WEI）
payAt                = 100                         //触发付款的阈值请求（单位）
dropAt               = 10000                       //触发断开连接的阈值（单位）
)

const (
	chequebookDeployRetries = 5
chequebookDeployDelay   = 1 * time.Second //重试之间的延迟
)

//localprofile将payprofile与*swap.params组合在一起
type LocalProfile struct {
	*swap.Params
	*PayProfile
}

//RemoteProfile将PayProfile与*swap.profile结合在一起。
type RemoteProfile struct {
	*swap.Profile
	*PayProfile
}

//PayProfile是相关支票簿和受益人选项的容器。
type PayProfile struct {
PublicKey   string         //与承诺的签署核对
Contract    common.Address //支票簿合同地址
Beneficiary common.Address //Swarm销售收入的收件人地址
	privateKey  *ecdsa.PrivateKey
	publicKey   *ecdsa.PublicKey
	owner       common.Address
	chbook      *chequebook.Chequebook
	lock        sync.RWMutex
}

//newdefaultswapparams使用默认值创建参数
func NewDefaultSwapParams() *LocalProfile {
	return &LocalProfile{
		PayProfile: &PayProfile{},
		Params: &swap.Params{
			Profile: &swap.Profile{
				BuyAt:  buyAt,
				SellAt: sellAt,
				PayAt:  uint(payAt),
				DropAt: uint(dropAt),
			},
			Strategy: &swap.Strategy{
				AutoCashInterval:     autoCashInterval,
				AutoCashThreshold:    autoCashThreshold,
				AutoDepositInterval:  autoDepositInterval,
				AutoDepositThreshold: autoDepositThreshold,
				AutoDepositBuffer:    autoDepositBuffer,
			},
		},
	}
}

//init这只能在所有配置选项（file、cmd line、env vars）之后设置。
//已经过评估
func (lp *LocalProfile) Init(contract common.Address, prvkey *ecdsa.PrivateKey) {
	pubkey := &prvkey.PublicKey

	lp.PayProfile = &PayProfile{
		PublicKey:   common.ToHex(crypto.FromECDSAPub(pubkey)),
		Contract:    contract,
		Beneficiary: crypto.PubkeyToAddress(*pubkey),
		privateKey:  prvkey,
		publicKey:   pubkey,
		owner:       crypto.PubkeyToAddress(*pubkey),
	}
}

//Newswp构造函数，参数
//*全球支票簿，承担部署服务和
//*余额处于缓冲状态。
//在netstore中调用swap.add（n）
//n>0发送块时调用=接收检索请求
//或者寄支票。
//n<0在接收数据块时调用=接收传递响应
//或者收到支票。
func NewSwap(localProfile *LocalProfile, remoteProfile *RemoteProfile, backend chequebook.Backend, proto swap.Protocol) (swapInstance *swap.Swap, err error) {
	var (
		ctx = context.TODO()
		ok  bool
		in  *chequebook.Inbox
		out *chequebook.Outbox
	)

	remotekey, err := crypto.UnmarshalPubkey(common.FromHex(remoteProfile.PublicKey))
	if err != nil {
		return nil, errors.New("invalid remote public key")
	}

//检查远程配置文件支票簿是否有效
//资不抵债支票簿自杀，因此将显示无效
//TODO:监视支票簿事件
	ok, err = chequebook.ValidateCode(ctx, backend, remoteProfile.Contract)
	if !ok {
		log.Info(fmt.Sprintf("invalid contract %v for peer %v: %v)", remoteProfile.Contract.Hex()[:8], proto, err))
	} else {
//远程配置文件合同有效，创建收件箱
		in, err = chequebook.NewInbox(localProfile.privateKey, remoteProfile.Contract, localProfile.Beneficiary, remotekey, backend)
		if err != nil {
			log.Warn(fmt.Sprintf("unable to set up inbox for chequebook contract %v for peer %v: %v)", remoteProfile.Contract.Hex()[:8], proto, err))
		}
	}

//检查LocalProfile支票簿合同是否有效
	ok, err = chequebook.ValidateCode(ctx, backend, localProfile.Contract)
	if !ok {
		log.Warn(fmt.Sprintf("unable to set up outbox for peer %v:  chequebook contract (owner: %v): %v)", proto, localProfile.owner.Hex(), err))
	} else {
		out = chequebook.NewOutbox(localProfile.Chequebook(), remoteProfile.Beneficiary)
	}

	pm := swap.Payment{
		In:    in,
		Out:   out,
		Buys:  out != nil,
		Sells: in != nil,
	}
	swapInstance, err = swap.New(localProfile.Params, pm, proto)
	if err != nil {
		return
	}
//握手中给定的远程配置文件（第一个）
	swapInstance.SetRemote(remoteProfile.Profile)
	var buy, sell string
	if swapInstance.Buys {
		buy = "purchase from peer enabled at " + remoteProfile.SellAt.String() + " wei/chunk"
	} else {
		buy = "purchase from peer disabled"
	}
	if swapInstance.Sells {
		sell = "selling to peer enabled at " + localProfile.SellAt.String() + " wei/chunk"
	} else {
		sell = "selling to peer disabled"
	}
	log.Warn(fmt.Sprintf("SWAP arrangement with <%v>: %v; %v)", proto, buy, sell))

	return
}

//从本地配置文件获取支票簿
func (lp *LocalProfile) Chequebook() *chequebook.Chequebook {
	defer lp.lock.Unlock()
	lp.lock.Lock()
	return lp.chbook
}

//私钥访问器
func (lp *LocalProfile) PrivateKey() *ecdsa.PrivateKey {
	return lp.privateKey
}

//func（self*localprofile）publickey（）*ecdsa.publickey_
//返回self.publickey
//}

//本地配置文件上的set key集的私钥和公钥
func (lp *LocalProfile) SetKey(prvkey *ecdsa.PrivateKey) {
	lp.privateKey = prvkey
	lp.publicKey = &prvkey.PublicKey
}

//setcheckbook包装支票簿初始化器并设置自动报告以覆盖支出。
func (lp *LocalProfile) SetChequebook(ctx context.Context, backend chequebook.Backend, path string) error {
	lp.lock.Lock()
	swapContract := lp.Contract
	lp.lock.Unlock()

	valid, err := chequebook.ValidateCode(ctx, backend, swapContract)
	if err != nil {
		return err
	} else if valid {
		return lp.newChequebookFromContract(path, backend)
	}
	return lp.deployChequebook(ctx, backend, path)
}

//deploychequebook部署本地配置文件支票簿
func (lp *LocalProfile) deployChequebook(ctx context.Context, backend chequebook.Backend, path string) error {
	opts := bind.NewKeyedTransactor(lp.privateKey)
	opts.Value = lp.AutoDepositBuffer
	opts.Context = ctx

	log.Info(fmt.Sprintf("Deploying new chequebook (owner: %v)", opts.From.Hex()))
	address, err := deployChequebookLoop(opts, backend)
	if err != nil {
		log.Error(fmt.Sprintf("unable to deploy new chequebook: %v", err))
		return err
	}
	log.Info(fmt.Sprintf("new chequebook deployed at %v (owner: %v)", address.Hex(), opts.From.Hex()))

//此时需要保存配置
	lp.lock.Lock()
	lp.Contract = address
	err = lp.newChequebookFromContract(path, backend)
	lp.lock.Unlock()
	if err != nil {
		log.Warn(fmt.Sprintf("error initialising cheque book (owner: %v): %v", opts.From.Hex(), err))
	}
	return err
}

//deploychequebookloop反复尝试部署支票簿。
func deployChequebookLoop(opts *bind.TransactOpts, backend chequebook.Backend) (addr common.Address, err error) {
	var tx *types.Transaction
	for try := 0; try < chequebookDeployRetries; try++ {
		if try > 0 {
			time.Sleep(chequebookDeployDelay)
		}
		if _, tx, _, err = contract.DeployChequebook(opts, backend); err != nil {
			log.Warn(fmt.Sprintf("can't send chequebook deploy tx (try %d): %v", try, err))
			continue
		}
		if addr, err = bind.WaitDeployed(opts.Context, backend, tx); err != nil {
			log.Warn(fmt.Sprintf("chequebook deploy error (try %d): %v", try, err))
			continue
		}
		return addr, nil
	}
	return addr, err
}

//newcheckbookfromcontract-从持久的JSON文件初始化支票簿或创建新的支票簿
//呼叫方持有锁
func (lp *LocalProfile) newChequebookFromContract(path string, backend chequebook.Backend) error {
	hexkey := common.Bytes2Hex(lp.Contract.Bytes())
	err := os.MkdirAll(filepath.Join(path, "chequebooks"), os.ModePerm)
	if err != nil {
		return fmt.Errorf("unable to create directory for chequebooks: %v", err)
	}

	chbookpath := filepath.Join(path, "chequebooks", hexkey+".json")
	lp.chbook, err = chequebook.LoadChequebook(chbookpath, lp.privateKey, backend, true)

	if err != nil {
		lp.chbook, err = chequebook.NewChequebook(chbookpath, lp.Contract, lp.privateKey, backend)
		if err != nil {
			log.Warn(fmt.Sprintf("unable to initialise chequebook (owner: %v): %v", lp.owner.Hex(), err))
			return fmt.Errorf("unable to initialise chequebook (owner: %v): %v", lp.owner.Hex(), err)
		}
	}

	lp.chbook.AutoDeposit(lp.AutoDepositInterval, lp.AutoDepositThreshold, lp.AutoDepositBuffer)
	log.Info(fmt.Sprintf("auto deposit ON for %v -> %v: interval = %v, threshold = %v, buffer = %v)", crypto.PubkeyToAddress(*(lp.publicKey)).Hex()[:8], lp.Contract.Hex()[:8], lp.AutoDepositInterval, lp.AutoDepositThreshold, lp.AutoDepositBuffer))

	return nil
}
