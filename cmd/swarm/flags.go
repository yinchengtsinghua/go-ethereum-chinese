
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
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

import cli "gopkg.in/urfave/cli.v1"

var (
	ChequebookAddrFlag = cli.StringFlag{
		Name:   "chequebook",
		Usage:  "chequebook contract address",
		EnvVar: SWARM_ENV_CHEQUEBOOK_ADDR,
	}
	SwarmAccountFlag = cli.StringFlag{
		Name:   "bzzaccount",
		Usage:  "Swarm account key file",
		EnvVar: SWARM_ENV_ACCOUNT,
	}
	SwarmListenAddrFlag = cli.StringFlag{
		Name:   "httpaddr",
		Usage:  "Swarm HTTP API listening interface",
		EnvVar: SWARM_ENV_LISTEN_ADDR,
	}
	SwarmPortFlag = cli.StringFlag{
		Name:   "bzzport",
		Usage:  "Swarm local http api port",
		EnvVar: SWARM_ENV_PORT,
	}
	SwarmNetworkIdFlag = cli.IntFlag{
		Name:   "bzznetworkid",
		Usage:  "Network identifier (integer, default 3=swarm testnet)",
		EnvVar: SWARM_ENV_NETWORK_ID,
	}
	SwarmSwapEnabledFlag = cli.BoolFlag{
		Name:   "swap",
		Usage:  "Swarm SWAP enabled (default false)",
		EnvVar: SWARM_ENV_SWAP_ENABLE,
	}
	SwarmSwapAPIFlag = cli.StringFlag{
		Name:   "swap-api",
		Usage:  "URL of the Ethereum API provider to use to settle SWAP payments",
		EnvVar: SWARM_ENV_SWAP_API,
	}
	SwarmSyncDisabledFlag = cli.BoolTFlag{
		Name:   "nosync",
		Usage:  "Disable swarm syncing",
		EnvVar: SWARM_ENV_SYNC_DISABLE,
	}
	SwarmSyncUpdateDelay = cli.DurationFlag{
		Name:   "sync-update-delay",
		Usage:  "Duration for sync subscriptions update after no new peers are added (default 15s)",
		EnvVar: SWARM_ENV_SYNC_UPDATE_DELAY,
	}
	SwarmMaxStreamPeerServersFlag = cli.IntFlag{
		Name:   "max-stream-peer-servers",
		Usage:  "Limit of Stream peer servers, 0 denotes unlimited",
		EnvVar: SWARM_ENV_MAX_STREAM_PEER_SERVERS,
Value:  10000, //
	}
	SwarmLightNodeEnabled = cli.BoolFlag{
		Name:   "lightnode",
		Usage:  "Enable Swarm LightNode (default false)",
		EnvVar: SWARM_ENV_LIGHT_NODE_ENABLE,
	}
	SwarmDeliverySkipCheckFlag = cli.BoolFlag{
		Name:   "delivery-skip-check",
		Usage:  "Skip chunk delivery check (default false)",
		EnvVar: SWARM_ENV_DELIVERY_SKIP_CHECK,
	}
	EnsAPIFlag = cli.StringSliceFlag{
		Name:   "ens-api",
		Usage:  "ENS API endpoint for a TLD and with contract address, can be repeated, format [tld:][contract-addr@]url",
		EnvVar: SWARM_ENV_ENS_API,
	}
	SwarmApiFlag = cli.StringFlag{
		Name:  "bzzapi",
		Usage: "Specifies the Swarm HTTP endpoint to connect to",
Value: "http://127.0.0.1:8500“，
	}
	SwarmRecursiveFlag = cli.BoolFlag{
		Name:  "recursive",
		Usage: "Upload directories recursively",
	}
	SwarmWantManifestFlag = cli.BoolTFlag{
		Name:  "manifest",
		Usage: "Automatic manifest upload (default true)",
	}
	SwarmUploadDefaultPath = cli.StringFlag{
		Name:  "defaultpath",
		Usage: "path to file served for empty url path (none)",
	}
	SwarmAccessGrantKeyFlag = cli.StringFlag{
		Name:  "grant-key",
		Usage: "grants a given public key access to an ACT",
	}
	SwarmAccessGrantKeysFlag = cli.StringFlag{
		Name:  "grant-keys",
		Usage: "grants a given list of public keys in the following file (separated by line breaks) access to an ACT",
	}
	SwarmUpFromStdinFlag = cli.BoolFlag{
		Name:  "stdin",
		Usage: "reads data to be uploaded from stdin",
	}
	SwarmUploadMimeType = cli.StringFlag{
		Name:  "mime",
		Usage: "Manually specify MIME type",
	}
	SwarmEncryptedFlag = cli.BoolFlag{
		Name:  "encrypt",
		Usage: "use encrypted upload",
	}
	SwarmAccessPasswordFlag = cli.StringFlag{
		Name:   "password",
		Usage:  "Password",
		EnvVar: SWARM_ACCESS_PASSWORD,
	}
	SwarmDryRunFlag = cli.BoolFlag{
		Name:  "dry-run",
		Usage: "dry-run",
	}
	CorsStringFlag = cli.StringFlag{
		Name:   "corsdomain",
		Usage:  "Domain on which to send Access-Control-Allow-Origin header (multiple domains can be supplied separated by a ',')",
		EnvVar: SWARM_ENV_CORS,
	}
	SwarmStorePath = cli.StringFlag{
		Name:   "store.path",
		Usage:  "Path to leveldb chunk DB (default <$GETH_ENV_DIR>/swarm/bzz-<$BZZ_KEY>/chunks)",
		EnvVar: SWARM_ENV_STORE_PATH,
	}
	SwarmStoreCapacity = cli.Uint64Flag{
		Name:   "store.size",
		Usage:  "Number of chunks (5M is roughly 20-25GB) (default 5000000)",
		EnvVar: SWARM_ENV_STORE_CAPACITY,
	}
	SwarmStoreCacheCapacity = cli.UintFlag{
		Name:   "store.cache.size",
		Usage:  "Number of recent chunks cached in memory (default 5000)",
		EnvVar: SWARM_ENV_STORE_CACHE_CAPACITY,
	}
	SwarmCompressedFlag = cli.BoolFlag{
		Name:  "compressed",
		Usage: "Prints encryption keys in compressed form",
	}
	SwarmFeedNameFlag = cli.StringFlag{
		Name:  "name",
		Usage: "User-defined name for the new feed, limited to 32 characters. If combined with topic, it will refer to a subtopic with this name",
	}
	SwarmFeedTopicFlag = cli.StringFlag{
		Name:  "topic",
		Usage: "User-defined topic this feed is tracking, hex encoded. Limited to 64 hexadecimal characters",
	}
	SwarmFeedManifestFlag = cli.StringFlag{
		Name:  "manifest",
		Usage: "Refers to the feed through a manifest",
	}
	SwarmFeedUserFlag = cli.StringFlag{
		Name:  "user",
		Usage: "Indicates the user who updates the feed",
	}
)
