
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

//此文件包含嵌入到
//GO测试。这样可以确保在out指南中发布的任何代码都不会中断
//意外地通过一些代码更新。如果一些API仍然发生变化，那么需要
//修改此文件，请将任何修改导入到开发人员的
//也可以浏览维基网页！

package guide

import (
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/core/types"
)

//测试帐户管理片段是否正常工作。
func TestAccountManagement(t *testing.T) {
//创建要使用的临时文件夹
	workdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Failed to create temporary work dir: %v", err)
	}
	defer os.RemoveAll(workdir)

//使用标准加密参数创建加密密钥库
	ks := keystore.NewKeyStore(filepath.Join(workdir, "keystore"), keystore.StandardScryptN, keystore.StandardScryptP)

//使用指定的加密密码创建新帐户
	newAcc, err := ks.NewAccount("Creation password")
	if err != nil {
		t.Fatalf("Failed to create new account: %v", err)
	}
//使用其他密码短语导出新创建的帐户。归还的人
//此方法调用的数据是一个JSON编码的加密密钥文件
	jsonAcc, err := ks.Export(newAcc, "Creation password", "Export password")
	if err != nil {
		t.Fatalf("Failed to export account: %v", err)
	}
//在本地密钥库中更新上面创建的帐户的密码短语
	if err := ks.Update(newAcc, "Creation password", "Update password"); err != nil {
		t.Fatalf("Failed to update account: %v", err)
	}
//从本地密钥库中删除上面更新的帐户
	if err := ks.Delete(newAcc, "Update password"); err != nil {
		t.Fatalf("Failed to delete account: %v", err)
	}
//导入回我们在上面导出（然后删除）的帐户
//又是一个新的密码
	if _, err := ks.Import(jsonAcc, "Export password", "Import password"); err != nil {
		t.Fatalf("Failed to import account: %v", err)
	}
//创建用于签署交易记录的新帐户
	signer, err := ks.NewAccount("Signer password")
	if err != nil {
		t.Fatalf("Failed to create signer account: %v", err)
	}
	tx, chain := new(types.Transaction), big.NewInt(1)

//用单一授权签署交易
	if _, err := ks.SignTxWithPassphrase(signer, "Signer password", tx, chain); err != nil {
		t.Fatalf("Failed to sign with passphrase: %v", err)
	}
//使用多个手动取消的授权签署交易
	if err := ks.Unlock(signer, "Signer password"); err != nil {
		t.Fatalf("Failed to unlock account: %v", err)
	}
	if _, err := ks.SignTx(signer, tx, chain); err != nil {
		t.Fatalf("Failed to sign with unlocked account: %v", err)
	}
	if err := ks.Lock(signer.Address); err != nil {
		t.Fatalf("Failed to lock account: %v", err)
	}
//使用多个自动取消的授权签署交易
	if err := ks.TimedUnlock(signer, "Signer password", time.Second); err != nil {
		t.Fatalf("Failed to time unlock account: %v", err)
	}
	if _, err := ks.SignTx(signer, tx, chain); err != nil {
		t.Fatalf("Failed to sign with time unlocked account: %v", err)
	}
}
