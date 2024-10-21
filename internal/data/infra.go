package data

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"

	"transmission-proxy/conf"
	"transmission-proxy/internal/domain"

	"github.com/dgraph-io/ristretto"
	gocache "github.com/eko/gocache/lib/v4/cache"
	ristrettostore "github.com/eko/gocache/store/ristretto/v4"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/google/wire"
	"github.com/hekmon/transmissionrpc/v3"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(NewInfra, NewAppDao, NewTorrentDao)

// PeerCacheSize Peer缓存大小
const PeerCacheSize = 1 << 20 // 1M内存

var (
	BanIPV4SetName = "trp_black_ipv4"
	BanIPV4Table   = &nftables.Table{
		Name:   "filter",
		Family: nftables.TableFamilyIPv4,
	}
	BanIPV4InputChain = &nftables.Chain{
		Table:    BanIPV4Table,
		Name:     "input",
		Hooknum:  nftables.ChainHookInput,
		Priority: nftables.ChainPriorityFilter,
		Type:     nftables.ChainTypeFilter,
	}
	BanIPV4OutputChain = &nftables.Chain{
		Table:    BanIPV4Table,
		Name:     "output",
		Hooknum:  nftables.ChainHookOutput,
		Priority: nftables.ChainPriorityFilter,
		Type:     nftables.ChainTypeFilter,
	}
	BanIPV4Set = &nftables.Set{
		Table:    BanIPV4Table,
		Name:     BanIPV4SetName,
		KeyType:  nftables.TypeIPAddr,
		Interval: false, // 是否使用区间匹配 (单个 IP 则为 false)
	}
	BanIPV4InputRule = &nftables.Rule{
		Table: BanIPV4Table,
		Chain: BanIPV4InputChain,
		Exprs: []expr.Any{
			&expr.Payload{
				DestRegister: 1,                             // 目标寄存器
				Base:         expr.PayloadBaseNetworkHeader, // 从网络层报头读取
				Offset:       12,                            // 目标地址偏移量 12 字节
				Len:          4,                             // 取总长 4 字节
			},
			&expr.Lookup{
				SourceRegister: 1,              // 存入寄存器1
				SetName:        BanIPV4SetName, // 指定集合名
			},
			&expr.Verdict{
				Kind: expr.VerdictDrop, // 丢弃数据包
			},
		},
	}
	BanIPV4OutputRule = &nftables.Rule{
		Table: BanIPV4Table,
		Chain: BanIPV4OutputChain,
		Exprs: []expr.Any{
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       16,
				Len:          4,
			},
			&expr.Lookup{
				SourceRegister: 1,
				SetName:        BanIPV4SetName,
			},
			&expr.Verdict{
				Kind: expr.VerdictDrop,
			},
		},
	}
	BanIPV6SetName = "trp_black_ipv6"
	BanIPV6Table   = &nftables.Table{
		Name:   "filter",
		Family: nftables.TableFamilyIPv6,
	}
	BanIPV6InputChain = &nftables.Chain{
		Table:    BanIPV6Table,
		Name:     "input",
		Hooknum:  nftables.ChainHookInput,
		Priority: nftables.ChainPriorityFilter,
		Type:     nftables.ChainTypeFilter,
	}
	BanIPV6OutputChain = &nftables.Chain{
		Table:    BanIPV6Table,
		Name:     "output",
		Hooknum:  nftables.ChainHookOutput,
		Priority: nftables.ChainPriorityFilter,
		Type:     nftables.ChainTypeFilter,
	}
	BanIPV6Set = &nftables.Set{
		Table:    BanIPV6Table,
		Name:     BanIPV6SetName,
		KeyType:  nftables.TypeIP6Addr,
		Interval: false, // 是否使用区间匹配 (单个 IP 则为 false)
	}
	BanIPV6InputRule = &nftables.Rule{
		Table: BanIPV6Table,
		Chain: BanIPV6InputChain,
		Exprs: []expr.Any{
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       8,
				Len:          16,
			},
			&expr.Lookup{
				SourceRegister: 1,              // 存入寄存器1
				SetName:        BanIPV6SetName, // 指定集合名
			},
			&expr.Verdict{
				Kind: expr.VerdictDrop, // 丢弃数据包
			},
		},
	}
	BanIPV6OutputRule = &nftables.Rule{
		Table: BanIPV6Table,
		Chain: BanIPV6OutputChain,
		Exprs: []expr.Any{
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       24,
				Len:          16,
			},
			&expr.Lookup{
				SourceRegister: 1,
				SetName:        BanIPV6SetName,
			},
			&expr.Verdict{
				Kind: expr.VerdictDrop,
			},
		},
	}
)

// Infra .
type Infra struct {
	DB  *sqlx.DB
	TR  *transmissionrpc.Client
	NFT *nftables.Conn

	// key: <hash:ip:port>
	Cache *gocache.Cache[*domain.Peer]

	stateRefreshInterval int64
}

// NewInfra .
func NewInfra(bootstrap *conf.Bootstrap, logger log.Logger) (*Infra, func(), error) {
	config := bootstrap.GetInfra()
	stateRefreshInterval := bootstrap.GetInfra().GetTr().GetRequestInterval().AsDuration().Seconds()

	ll := log.NewHelper(logger)

	dbPath := config.GetDatabase().GetPath()
	// 判断文件是否存在
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		// 文件不存在，创建一个新的 SQLite 数据库文件
		file, err := os.Create(dbPath)
		if err != nil {
			return nil, nil, err
		}
		_ = file.Close()
		ll.Debugf("数据库文件创建成功: %s", dbPath)
	}

	ll.Debugf("读取数据库文件: %s", dbPath)
	db, err := sqlx.Connect("sqlite3", dbPath)
	if err != nil {
		return nil, nil, err
	}

	endpoint, err := url.Parse(config.GetTr().GetRpcUrl())
	if err != nil {
		return nil, nil, err
	}
	tr, err := transmissionrpc.New(endpoint, nil)
	if err != nil {
		return nil, nil, err
	}

	ok, rpcVersion, rpcMinimumVersion, err := tr.RPCVersion(context.Background())
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, errors.New(
			fmt.Sprintf("远程传输 RPC 版本 (v%d) 与传输库 (v%d) 不兼容：远程至少需要 v%d",
				rpcVersion, transmissionrpc.RPCVersion, rpcMinimumVersion),
		)
	}
	ll.Infof("远程传输 RPC 版本: v%d", rpcVersion)

	// 创建 nftables 句柄
	nft, err := nftables.New()
	if err != nil {
		return nil, nil, err
	}
	// 创建新的表
	BanIPV4Table = nft.AddTable(BanIPV4Table)
	// 创建链
	BanIPV4InputChain = nft.AddChain(BanIPV4InputChain)
	BanIPV4OutputChain = nft.AddChain(BanIPV4OutputChain)
	// 创建Set表
	err = nft.AddSet(BanIPV4Set, nil)
	if err != nil {
		return nil, nil, err
	}
	// 创建规则
	BanIPV4InputRule = nft.AddRule(BanIPV4InputRule)
	BanIPV4OutputRule = nft.AddRule(BanIPV4OutputRule)

	// 创建新的表
	BanIPV6Table = nft.AddTable(BanIPV6Table)
	// 创建链
	BanIPV6InputChain = nft.AddChain(BanIPV6InputChain)
	BanIPV6OutputChain = nft.AddChain(BanIPV6OutputChain)
	// 创建Set表
	err = nft.AddSet(BanIPV6Set, nil)
	if err != nil {
		return nil, nil, err
	}
	// 创建规则
	BanIPV6InputRule = nft.AddRule(BanIPV6InputRule)
	BanIPV6OutputRule = nft.AddRule(BanIPV6OutputRule)
	// 提交更改
	if err := nft.Flush(); err != nil {
		return nil, nil, err
	}

	// 创建缓存
	ristrettoCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 50000,         // 缓存数量
		MaxCost:     PeerCacheSize, // 最大缓存容量(字节, 1M内存)
		BufferItems: 64,            // number of keys per Get buffer.
	})
	if err != nil {
		return nil, nil, err
	}
	ristrettoStore := ristrettostore.NewRistretto(ristrettoCache)
	cache := gocache.New[*domain.Peer](ristrettoStore)

	infra := &Infra{
		DB:                   db,
		TR:                   tr,
		NFT:                  nft,
		Cache:                cache,
		stateRefreshInterval: int64(stateRefreshInterval),
	}

	cleanup := func() {
		var err error
		ll.Info("closing the infra resources")
		nft.DelChain(BanIPV4InputChain)
		nft.DelChain(BanIPV4OutputChain)
		nft.DelChain(BanIPV6InputChain)
		nft.DelChain(BanIPV6OutputChain)
		nft.DelSet(BanIPV4Set)
		nft.DelSet(BanIPV6Set)
		nft.DelTable(BanIPV4Table)
		nft.DelTable(BanIPV6Table)

		if err = nft.Flush(); err != nil {
			ll.Errorf("clean NFT sending error: %v", err)
		}

		if err = db.Close(); err != nil {
			ll.Errorf("clean Database sending error: %v", err)
		}

		ll.Info("completion of Infra resource closure")
	}
	return infra, cleanup, nil
}
