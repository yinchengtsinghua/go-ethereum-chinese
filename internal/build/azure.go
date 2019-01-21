
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

package build

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/Azure/azure-storage-blob-go/2018-03-28/azblob"
)

//azureBlobstoreConfig是一个身份验证和配置结构，其中包含
//Azure SDK与中的speicifc容器交互所需的数据
//博客商店。
type AzureBlobstoreConfig struct {
Account   string //帐户名称以授权API请求
Token     string //上述帐户的访问令牌
Container string //要将文件上载到的Blob容器
}

//AzureBlobstoreUpload uploads a local file to the Azure Blob Storage. 注意，这个
//方法假定最大文件大小为64MB（Azure限制）。较大的文件将
//需要实现多API调用方法。
//
//请参阅：https://msdn.microsoft.com/en-us/library/azure/dd179451.aspx anchor_3
func AzureBlobstoreUpload(path string, name string, config AzureBlobstoreConfig) error {
	if *DryRunFlag {
		fmt.Printf("would upload %q to %s/%s/%s\n", path, config.Account, config.Container, name)
		return nil
	}
//Create an authenticated client against the Azure cloud
	credential := azblob.NewSharedKeyCredential(config.Account, config.Token)
	pipeline := azblob.NewPipeline(credential, azblob.PipelineOptions{})

u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net“，配置帐户）
	service := azblob.NewServiceURL(*u, pipeline)

	container := service.NewContainerURL(config.Container)
	blockblob := container.NewBlockBlobURL(name)

//将要上载的文件传输到指定的blobstore容器中
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = blockblob.Upload(context.Background(), in, azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	return err
}

//AzureBlobstoreList lists all the files contained within an azure blobstore.
func AzureBlobstoreList(config AzureBlobstoreConfig) ([]azblob.BlobItem, error) {
	credential := azblob.NewSharedKeyCredential(config.Account, config.Token)
	pipeline := azblob.NewPipeline(credential, azblob.PipelineOptions{})

u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net“，配置帐户）
	service := azblob.NewServiceURL(*u, pipeline)

//列出容器中的所有blob并将其返回
	container := service.NewContainerURL(config.Container)

	res, err := container.ListBlobsFlatSegment(context.Background(), azblob.Marker{}, azblob.ListBlobsSegmentOptions{
MaxResults: 1024 * 1024 * 1024, //是的，把它们都拿出来
	})
	if err != nil {
		return nil, err
	}
	return res.Segment.BlobItems, nil
}

//azureblobstorelete迭代要删除的文件列表并删除它们
//从墓碑上。
func AzureBlobstoreDelete(config AzureBlobstoreConfig, blobs []azblob.BlobItem) error {
	if *DryRunFlag {
		for _, blob := range blobs {
			fmt.Printf("would delete %s (%s) from %s/%s\n", blob.Name, blob.Properties.LastModified, config.Account, config.Container)
		}
		return nil
	}
//Create an authenticated client against the Azure cloud
	credential := azblob.NewSharedKeyCredential(config.Account, config.Token)
	pipeline := azblob.NewPipeline(credential, azblob.PipelineOptions{})

u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net“，配置帐户）
	service := azblob.NewServiceURL(*u, pipeline)

	container := service.NewContainerURL(config.Container)

//迭代这些blob并删除它们
	for _, blob := range blobs {
		blockblob := container.NewBlockBlobURL(blob.Name)
		if _, err := blockblob.Delete(context.Background(), azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{}); err != nil {
			return err
		}
	}
	return nil
}
