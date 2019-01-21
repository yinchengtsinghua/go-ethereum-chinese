
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2017 Go Ethereum作者
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

package keystore

import (
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/core/types"
)

//keystorewallet实现原始帐户的accounts.wallet接口
//密钥存储区。
type keystoreWallet struct {
account  accounts.Account //钱包里有一个账户
keystore *KeyStore        //帐户来源的密钥库
}

//url实现accounts.wallet，返回帐户的url。
func (w *keystoreWallet) URL() accounts.URL {
	return w.account.URL
}

//状态实现accounts.wallet，返回
//密钥存储钱包是否已解锁。
func (w *keystoreWallet) Status() (string, error) {
	w.keystore.mu.RLock()
	defer w.keystore.mu.RUnlock()

	if _, ok := w.keystore.unlocked[w.account.Address]; ok {
		return "Unlocked", nil
	}
	return "Locked", nil
}

//打开工具帐户。钱包，但是普通钱包的一个noop，因为那里
//访问帐户列表不需要连接或解密步骤。
func (w *keystoreWallet) Open(passphrase string) error { return nil }

//close实现了账户。钱包，但它是普通钱包的一个noop，因为
//不是有意义的开放式操作。
func (w *keystoreWallet) Close() error { return nil }

//帐户实现帐户。钱包，返回包含
//普通的Kestore钱包中包含的单个帐户。
func (w *keystoreWallet) Accounts() []accounts.Account {
	return []accounts.Account{w.account}
}

//包含implements accounts.wallet，返回特定帐户是否为
//或未被此钱包实例包装。
func (w *keystoreWallet) Contains(account accounts.Account) bool {
	return account.Address == w.account.Address && (account.URL == (accounts.URL{}) || account.URL == w.account.URL)
}

//派生实现了accounts.wallet，但对于普通的钱包来说是一个noop，因为
//对于普通的密钥存储帐户，不存在分层帐户派生的概念。
func (w *keystoreWallet) Derive(path accounts.DerivationPath, pin bool) (accounts.Account, error) {
	return accounts.Account{}, accounts.ErrNotSupported
}

//Selfderive实现了accounts.wallet，但对于普通的钱包来说是一个noop，因为
//对于普通密钥库帐户，没有层次结构帐户派生的概念。
func (w *keystoreWallet) SelfDerive(base accounts.DerivationPath, chain ethereum.ChainStateReader) {}

//sign hash实现accounts.wallet，尝试用
//给定的帐户。如果钱包没有包裹这个特定的账户，
//返回错误以避免帐户泄漏（即使在理论上我们可能
//能够通过我们的共享密钥库后端进行签名）。
func (w *keystoreWallet) SignHash(account accounts.Account, hash []byte) ([]byte, error) {
//确保请求的帐户包含在
	if !w.Contains(account) {
		return nil, accounts.ErrUnknownAccount
	}
//帐户似乎有效，请求密钥库签名
	return w.keystore.SignHash(account, hash)
}

//signtx实现accounts.wallet，尝试签署给定的交易
//与给定的帐户。如果钱包没有包裹这个特定的账户，
//返回一个错误以避免帐户泄漏（即使在理论上我们可以
//能够通过我们的共享密钥库后端进行签名）。
func (w *keystoreWallet) SignTx(account accounts.Account, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
//确保请求的帐户包含在
	if !w.Contains(account) {
		return nil, accounts.ErrUnknownAccount
	}
//帐户似乎有效，请求密钥库签名
	return w.keystore.SignTx(account, tx, chainID)
}

//signhashwithpassphrase实现accounts.wallet，尝试对
//使用密码短语作为额外身份验证的给定帐户的给定哈希。
func (w *keystoreWallet) SignHashWithPassphrase(account accounts.Account, passphrase string, hash []byte) ([]byte, error) {
//确保请求的帐户包含在
	if !w.Contains(account) {
		return nil, accounts.ErrUnknownAccount
	}
//帐户似乎有效，请求密钥库签名
	return w.keystore.SignHashWithPassphrase(account, passphrase, hash)
}

//signtxwithpassphrase实现accounts.wallet，尝试对给定的
//使用密码短语作为额外身份验证的给定帐户的事务。
func (w *keystoreWallet) SignTxWithPassphrase(account accounts.Account, passphrase string, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
//确保请求的帐户包含在
	if !w.Contains(account) {
		return nil, accounts.ErrUnknownAccount
	}
//帐户似乎有效，请求密钥库签名
	return w.keystore.SignTxWithPassphrase(account, passphrase, tx, chainID)
}
