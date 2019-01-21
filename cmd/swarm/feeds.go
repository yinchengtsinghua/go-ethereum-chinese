
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
//此文件是Go以太坊的一部分。
//
//Go以太坊是免费软件：您可以重新发布和/或修改它
//根据GNU通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊的分布希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU通用公共许可证了解更多详细信息。
//
//你应该已经收到一份GNU通用公共许可证的副本
//一起去以太坊吧。如果没有，请参见<http://www.gnu.org/licenses/>。

//命令feed允许用户创建和更新签名的swarm feed
package main

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum/go-ethereum/cmd/utils"
	swarm "github.com/ethereum/go-ethereum/swarm/api/client"
	"github.com/ethereum/go-ethereum/swarm/storage/feed"
	"gopkg.in/urfave/cli.v1"
)

var feedCommand = cli.Command{
	CustomHelpTemplate: helpTemplate,
	Name:               "feed",
	Usage:              "(Advanced) Create and update Swarm Feeds",
	ArgsUsage:          "<create|update|info>",
	Description:        "Works with Swarm Feeds",
	Subcommands: []cli.Command{
		{
			Action:             feedCreateManifest,
			CustomHelpTemplate: helpTemplate,
			Name:               "create",
			Usage:              "creates and publishes a new feed manifest",
			Description: `creates and publishes a new feed manifest pointing to a specified user's updates about a particular topic.
					The feed topic can be built in the following ways:
					* use --topic to set the topic to an arbitrary binary hex string.
					* use --name to set the topic to a human-readable name.
					    For example --name could be set to "profile-picture", meaning this feed allows to get this user's current profile picture.
					* use both --topic and --name to create named subtopics. 
						For example, --topic could be set to an Ethereum contract address and --name could be set to "comments", meaning
						this feed tracks a discussion about that contract.
					The --user flag allows to have this manifest refer to a user other than yourself. If not specified,
					it will then default to your local account (--bzzaccount)`,
			Flags: []cli.Flag{SwarmFeedNameFlag, SwarmFeedTopicFlag, SwarmFeedUserFlag},
		},
		{
			Action:             feedUpdate,
			CustomHelpTemplate: helpTemplate,
			Name:               "update",
			Usage:              "updates the content of an existing Swarm Feed",
			ArgsUsage:          "<0x Hex data>",
			Description: `publishes a new update on the specified topic
					The feed topic can be built in the following ways:
					* use --topic to set the topic to an arbitrary binary hex string.
					* use --name to set the topic to a human-readable name.
					    For example --name could be set to "profile-picture", meaning this feed allows to get this user's current profile picture.
					* use both --topic and --name to create named subtopics. 
						For example, --topic could be set to an Ethereum contract address and --name could be set to "comments", meaning
						this feed tracks a discussion about that contract.
					
					If you have a manifest, you can specify it with --manifest to refer to the feed,
					instead of using --topic / --name
					`,
			Flags: []cli.Flag{SwarmFeedManifestFlag, SwarmFeedNameFlag, SwarmFeedTopicFlag},
		},
		{
			Action:             feedInfo,
			CustomHelpTemplate: helpTemplate,
			Name:               "info",
			Usage:              "obtains information about an existing Swarm feed",
			Description: `obtains information about an existing Swarm feed
					The topic can be specified directly with the --topic flag as an hex string
					If no topic is specified, the default topic (zero) will be used
					The --name flag can be used to specify subtopics with a specific name.
					The --user flag allows to refer to a user other than yourself. If not specified,
					it will then default to your local account (--bzzaccount)
					If you have a manifest, you can specify it with --manifest instead of --topic / --name / ---user
					to refer to the feed`,
			Flags: []cli.Flag{SwarmFeedManifestFlag, SwarmFeedNameFlag, SwarmFeedTopicFlag, SwarmFeedUserFlag},
		},
	},
}

func NewGenericSigner(ctx *cli.Context) feed.Signer {
	return feed.NewGenericSigner(getPrivKey(ctx))
}

func getTopic(ctx *cli.Context) (topic feed.Topic) {
	var name = ctx.String(SwarmFeedNameFlag.Name)
	var relatedTopic = ctx.String(SwarmFeedTopicFlag.Name)
	var relatedTopicBytes []byte
	var err error

	if relatedTopic != "" {
		relatedTopicBytes, err = hexutil.Decode(relatedTopic)
		if err != nil {
			utils.Fatalf("Error parsing topic: %s", err)
		}
	}

	topic, err = feed.NewTopic(name, relatedTopicBytes)
	if err != nil {
		utils.Fatalf("Error parsing topic: %s", err)
	}
	return topic
}

//
//swarm feed update<manifest address or ens domain><0x hexdata>[--multihash=false]
//

func feedCreateManifest(ctx *cli.Context) {
	var (
		bzzapi = strings.TrimRight(ctx.GlobalString(SwarmApiFlag.Name), "/")
		client = swarm.NewClient(bzzapi)
	)

	newFeedUpdateRequest := feed.NewFirstRequest(getTopic(ctx))
	newFeedUpdateRequest.Feed.User = feedGetUser(ctx)

	manifestAddress, err := client.CreateFeedWithManifest(newFeedUpdateRequest)
	if err != nil {
		utils.Fatalf("Error creating feed manifest: %s", err.Error())
		return
	}
fmt.Println(manifestAddress) //

}

func feedUpdate(ctx *cli.Context) {
	args := ctx.Args()

	var (
		bzzapi                  = strings.TrimRight(ctx.GlobalString(SwarmApiFlag.Name), "/")
		client                  = swarm.NewClient(bzzapi)
		manifestAddressOrDomain = ctx.String(SwarmFeedManifestFlag.Name)
	)

	if len(args) < 1 {
		fmt.Println("Incorrect number of arguments")
		cli.ShowCommandHelpAndExit(ctx, "update", 1)
		return
	}

	signer := NewGenericSigner(ctx)

	data, err := hexutil.Decode(args[0])
	if err != nil {
		utils.Fatalf("Error parsing data: %s", err.Error())
		return
	}

	var updateRequest *feed.Request
	var query *feed.Query

	if manifestAddressOrDomain == "" {
		query = new(feed.Query)
		query.User = signer.Address()
		query.Topic = getTopic(ctx)
	}

//
	updateRequest, err = client.GetFeedRequest(query, manifestAddressOrDomain)
	if err != nil {
		utils.Fatalf("Error retrieving feed status: %s", err.Error())
	}

//
	if updateRequest.User != signer.Address() {
		utils.Fatalf("Signer address does not match the update request")
	}

//设置新数据
	updateRequest.SetData(data)

//
	if err = updateRequest.Sign(signer); err != nil {
		utils.Fatalf("Error signing feed update: %s", err.Error())
	}

//更新后
	err = client.UpdateFeed(updateRequest)
	if err != nil {
		utils.Fatalf("Error updating feed: %s", err.Error())
		return
	}
}

func feedInfo(ctx *cli.Context) {
	var (
		bzzapi                  = strings.TrimRight(ctx.GlobalString(SwarmApiFlag.Name), "/")
		client                  = swarm.NewClient(bzzapi)
		manifestAddressOrDomain = ctx.String(SwarmFeedManifestFlag.Name)
	)

	var query *feed.Query
	if manifestAddressOrDomain == "" {
		query = new(feed.Query)
		query.Topic = getTopic(ctx)
		query.User = feedGetUser(ctx)
	}

	metadata, err := client.GetFeedRequest(query, manifestAddressOrDomain)
	if err != nil {
		utils.Fatalf("Error retrieving feed metadata: %s", err.Error())
		return
	}
	encodedMetadata, err := metadata.MarshalJSON()
	if err != nil {
		utils.Fatalf("Error encoding metadata to JSON for display:%s", err)
	}
	fmt.Println(string(encodedMetadata))
}

func feedGetUser(ctx *cli.Context) common.Address {
	var user = ctx.String(SwarmFeedUserFlag.Name)
	if user != "" {
		return common.HexToAddress(user)
	}
	pk := getPrivKey(ctx)
	if pk == nil {
		utils.Fatalf("Cannot read private key. Must specify --user or --bzzaccount")
	}
	return crypto.PubkeyToAddress(pk.PublicKey)

}
