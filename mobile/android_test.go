
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

package geth

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/internal/build"
)

//ANDROIDTestCclass是一个Java类，用于对Android进行一些轻量级测试
//绑定。目标不是测试每个单独的功能，而是
//捕获中断的API和/或实现更改。
const androidTestClass = `
package go;

import android.test.InstrumentationTestCase;
import android.test.MoreAsserts;

import java.math.BigInteger;
import java.util.Arrays;

import org.ethereum.geth.*;

public class AndroidTest extends InstrumentationTestCase {
	public AndroidTest() {}

	public void testAccountManagement() {
//使用轻量级加密参数创建加密密钥库。
		KeyStore ks = new KeyStore(getInstrumentation().getContext().getFilesDir() + "/keystore", Geth.LightScryptN, Geth.LightScryptP);

		try {
//使用指定的加密密码创建新帐户。
			Account newAcc = ks.newAccount("Creation password");

//使用其他密码短语导出新创建的帐户。归还的人
//此方法调用中的数据是一个JSON编码的加密密钥文件。
			byte[] jsonAcc = ks.exportKey(newAcc, "Creation password", "Export password");

//在本地密钥库中更新上面创建的帐户的密码。
			ks.updateAccount(newAcc, "Creation password", "Update password");

//从本地密钥库中删除上面更新的帐户。
			ks.deleteAccount(newAcc, "Update password");

//导入回我们在上面导出（然后删除）的帐户
//又是一个新的密码。
			Account impAcc = ks.importKey(jsonAcc, "Export password", "Import password");

//创建用于签署交易记录的新帐户
			Account signer = ks.newAccount("Signer password");

			Transaction tx = new Transaction(
				1, new Address("0x0000000000000000000000000000000000000000"),
new BigInt(0), 0, new BigInt(1), null); //随机空事务
BigInt chain = new BigInt(1); //主网链标识符

//用单一授权签署交易
			Transaction signed = ks.signTxPassphrase(signer, "Signer password", tx, chain);

//使用多个手动取消的授权签署交易
			ks.unlock(signer, "Signer password");
			signed = ks.signTx(signer, tx, chain);
			ks.lock(signer.getAddress());

//使用多个自动取消的授权签署交易
			ks.timedUnlock(signer, "Signer password", 1000000000);
			signed = ks.signTx(signer, tx, chain);
		} catch (Exception e) {
			fail(e.toString());
		}
	}

	public void testInprocNode() {
		Context ctx = new Context();

		try {
//启动新的进程内节点
			Node node = new Node(getInstrumentation().getContext().getFilesDir() + "/.ethereum", new NodeConfig());
			node.start();

//通过函数调用检索一些数据（我们并不真正关心结果）
			NodeInfo info = node.getNodeInfo();
			info.getName();
			info.getListenerAddress();
			info.getProtocols();

//通过API检索一些数据（我们并不真正关心结果）
			EthereumClient ec = node.getEthereumClient();
			ec.getBlockByNumber(ctx, -1).getNumber();

			NewHeadHandler handler = new NewHeadHandler() {
				@Override public void onError(String error)          {}
				@Override public void onNewHead(final Header header) {}
			};
			ec.subscribeNewHead(ctx, handler,  16);
		} catch (Exception e) {
			fail(e.toString());
		}
	}

//恢复事务签名者对homestead和eip155都有效的测试
//签名也是。Go Ethereum问题的回归测试14599。
	public void testIssue14599() {
		try {
			byte[] preEIP155RLP = new BigInteger("f901fc8032830138808080b901ae60056013565b6101918061001d6000396000f35b3360008190555056006001600060e060020a6000350480630a874df61461003a57806341c0e1b514610058578063a02b161e14610066578063dbbdf0831461007757005b610045600435610149565b80600160a060020a031660005260206000f35b610060610161565b60006000f35b6100716004356100d4565b60006000f35b61008560043560243561008b565b60006000f35b600054600160a060020a031632600160a060020a031614156100ac576100b1565b6100d0565b8060018360005260205260406000208190555081600060005260206000a15b5050565b600054600160a060020a031633600160a060020a031614158015610118575033600160a060020a0316600182600052602052604060002054600160a060020a031614155b61012157610126565b610146565b600060018260005260205260406000208190555080600060005260206000a15b50565b60006001826000526020526040600020549050919050565b600054600160a060020a031633600160a060020a0316146101815761018f565b600054600160a060020a0316ff5b561ca0c5689ed1ad124753d54576dfb4b571465a41900a1dff4058d8adf16f752013d0a01221cbd70ec28c94a3b55ec771bcbc70778d6ee0b51ca7ea9514594c861b1884", 16).toByteArray();
			preEIP155RLP = Arrays.copyOfRange(preEIP155RLP, 1, preEIP155RLP.length);

			byte[] postEIP155RLP = new BigInteger("f86b80847735940082520894ef5bbb9bba2e1ca69ef81b23a8727d889f3ef0a1880de0b6b3a7640000802ba06fef16c44726a102e6d55a651740636ef8aec6df3ebf009e7b0c1f29e4ac114aa057e7fbc69760b522a78bb568cfc37a58bfdcf6ea86cb8f9b550263f58074b9cc", 16).toByteArray();
			postEIP155RLP = Arrays.copyOfRange(postEIP155RLP, 1, postEIP155RLP.length);

			Transaction preEIP155 =  new Transaction(preEIP155RLP);
			Transaction postEIP155 = new Transaction(postEIP155RLP);

preEIP155.getFrom(null);           //宅基地应接受宅基地
preEIP155.getFrom(new BigInt(4));  //EIP155应接受宅基地（缺少链ID）
postEIP155.getFrom(new BigInt(4)); //EIP155应接受EIP 155

			try {
				postEIP155.getFrom(null);
				fail("EIP155 transaction accepted by Homestead");
			} catch (Exception e) {}
		} catch (Exception e) {
			fail(e.toString());
		}
	}
}
`

//TESTALIDLE运行上面指定的Android Java测试类。
//
//这需要path中的gradle命令和路径可用的android sdk
//通过android_home环境变量。为了成功地运行测试，Android
//设备还必须在启用调试的情况下可用。
//
//此方法改编自golang.org/x/mobile/bind/java/seq_test.go/runtest。
func TestAndroid(t *testing.T) {
//完全跳过Windows上的测试
	if runtime.GOOS == "windows" {
		t.Skip("cannot test Android bindings on Windows, skipping")
	}
//确保已安装所有Android工具
	if _, err := exec.Command("which", "gradle").CombinedOutput(); err != nil {
		t.Skip("command gradle not found, skipping")
	}
	if sdk := os.Getenv("ANDROID_HOME"); sdk == "" {
//Android SDK没有明确给出，请尝试自动解决
		autopath := filepath.Join(os.Getenv("HOME"), "Android", "Sdk")
		if _, err := os.Stat(autopath); err != nil {
			t.Skip("ANDROID_HOME environment var not set, skipping")
		}
		os.Setenv("ANDROID_HOME", autopath)
	}
	if _, err := exec.Command("which", "gomobile").CombinedOutput(); err != nil {
		t.Log("gomobile missing, installing it...")
		if out, err := exec.Command("go", "get", "golang.org/x/mobile/cmd/gomobile").CombinedOutput(); err != nil {
			t.Fatalf("install failed: %v\n%s", err, string(out))
		}
		t.Log("initializing gomobile...")
		start := time.Now()
		if _, err := exec.Command("gomobile", "init").CombinedOutput(); err != nil {
			t.Fatalf("initialization failed: %v", err)
		}
		t.Logf("initialization took %v", time.Since(start))
	}
//创建并切换到临时工作区
	workspace, err := ioutil.TempDir("", "geth-android-")
	if err != nil {
		t.Fatalf("failed to create temporary workspace: %v", err)
	}
	defer os.RemoveAll(workspace)

	pwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("failed to switch to temporary workspace: %v", err)
	}
	defer os.Chdir(pwd)

//创建Android项目的框架
	for _, dir := range []string{"src/main", "src/androidTest/java/org/ethereum/gethtest", "libs"} {
		err = os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			t.Fatal(err)
		}
	}
//为geth生成移动绑定并添加Tester类
	gobind := exec.Command("gomobile", "bind", "-javapkg", "org.ethereum", "github.com/ethereum/go-ethereum/mobile")
	if output, err := gobind.CombinedOutput(); err != nil {
		t.Logf("%s", output)
		t.Fatalf("failed to run gomobile bind: %v", err)
	}
	build.CopyFile(filepath.Join("libs", "geth.aar"), "geth.aar", os.ModePerm)

	if err = ioutil.WriteFile(filepath.Join("src", "androidTest", "java", "org", "ethereum", "gethtest", "AndroidTest.java"), []byte(androidTestClass), os.ModePerm); err != nil {
		t.Fatalf("failed to write Android test class: %v", err)
	}
//完成项目创建并通过Gradle运行测试
	if err = ioutil.WriteFile(filepath.Join("src", "main", "AndroidManifest.xml"), []byte(androidManifest), os.ModePerm); err != nil {
		t.Fatalf("failed to write Android manifest: %v", err)
	}
	if err = ioutil.WriteFile("build.gradle", []byte(gradleConfig), os.ModePerm); err != nil {
		t.Fatalf("failed to write gradle build file: %v", err)
	}
	if output, err := exec.Command("gradle", "connectedAndroidTest").CombinedOutput(); err != nil {
		t.Logf("%s", output)
		t.Errorf("failed to run gradle test: %v", err)
	}
}

const androidManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android“
          package="org.ethereum.gethtest"
	  android:versionCode="1"
	  android:versionName="1.0">

		<uses-permission android:name="android.permission.INTERNET" />
</manifest>`

const gradleConfig = `buildscript {
    repositories {
        jcenter()
    }
    dependencies {
        classpath 'com.android.tools.build:gradle:2.2.3'
    }
}
allprojects {
    repositories { jcenter() }
}
apply plugin: 'com.android.library'
android {
    compileSdkVersion 'android-19'
    buildToolsVersion '21.1.2'
    defaultConfig { minSdkVersion 15 }
}
repositories {
    flatDir { dirs 'libs' }
}
dependencies {
    compile 'com.android.support:appcompat-v7:19.0.0'
    compile(name: "geth", ext: "aar")
}
`
