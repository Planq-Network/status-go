package node

import (
	"errors"
	"fmt"

	logging "github.com/ipfs/go-log"

	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/p2p/enode"

	"github.com/status-im/status-go/appmetrics"
	"github.com/status-im/status-go/common"
	gethbridge "github.com/status-im/status-go/eth-node/bridge/geth"
	"github.com/status-im/status-go/eth-node/types"
	"github.com/status-im/status-go/logutils"
	"github.com/status-im/status-go/mailserver"
	"github.com/status-im/status-go/multiaccounts/accounts"
	"github.com/status-im/status-go/params"
	"github.com/status-im/status-go/rpc"
	accountssvc "github.com/status-im/status-go/services/accounts"
	appmetricsservice "github.com/status-im/status-go/services/appmetrics"
	"github.com/status-im/status-go/services/browsers"
	"github.com/status-im/status-go/services/ext"
	localnotifications "github.com/status-im/status-go/services/local-notifications"
	"github.com/status-im/status-go/services/mailservers"
	"github.com/status-im/status-go/services/nodebridge"
	"github.com/status-im/status-go/services/peer"
	"github.com/status-im/status-go/services/permissions"
	"github.com/status-im/status-go/services/personal"
	"github.com/status-im/status-go/services/rpcfilters"
	"github.com/status-im/status-go/services/rpcstats"
	"github.com/status-im/status-go/services/subscriptions"
	"github.com/status-im/status-go/services/wakuext"
	"github.com/status-im/status-go/services/wakuv2ext"
	"github.com/status-im/status-go/services/wallet"
	"github.com/status-im/status-go/timesource"
	"github.com/status-im/status-go/waku"
	wakucommon "github.com/status-im/status-go/waku/common"
	"github.com/status-im/status-go/wakuv2"
)

var (
	// ErrWakuClearIdentitiesFailure clearing whisper identities has failed.
	ErrWakuClearIdentitiesFailure = errors.New("failed to clear waku identities")
)

func (b *StatusNode) initServices(config *params.NodeConfig) error {
	accountsFeed := &event.Feed{}

	services := []common.StatusService{}
	services = appendIf(config.UpstreamConfig.Enabled, services, b.rpcFiltersService())
	services = append(services, b.subscriptionService())
	services = append(services, b.rpcStatsService())
	services = append(services, b.appmetricsService())
	services = append(services, b.peerService())
	services = append(services, b.personalService())
	services = appendIf(config.EnableNTPSync, services, b.timeSource())
	services = appendIf(b.appDB != nil && b.multiaccountsDB != nil, services, b.accountsService(accountsFeed))
	services = appendIf(config.BrowsersConfig.Enabled, services, b.browsersService())
	services = appendIf(config.PermissionsConfig.Enabled, services, b.permissionsService())
	services = appendIf(config.MailserversConfig.Enabled, services, b.mailserversService())
	if config.WakuConfig.Enabled {
		wakuService, err := b.wakuService(&config.WakuConfig, &config.ClusterConfig)
		if err != nil {
			return err
		}

		services = append(services, wakuService)

		wakuext, err := b.wakuExtService(config)
		if err != nil {
			return err
		}

		b.wakuExtSrvc = wakuext

		services = append(services, wakuext)
	}

	if config.WakuV2Config.Enabled {
		waku2Service, err := b.wakuV2Service(config.NodeKey, &config.WakuV2Config, &config.ClusterConfig)
		if err != nil {
			return err
		}
		services = append(services, waku2Service)

		wakuext, err := b.wakuV2ExtService(config)
		if err != nil {
			return err
		}

		b.wakuV2ExtSrvc = wakuext

		services = append(services, wakuext)
	}

	b.log.Info("WAKU ENABLED")

	if config.WalletConfig.Enabled {
		walletService := b.walletService(config.NetworkID, accountsFeed)
		b.log.Info("SETTING REPC CLIETN")
		b.walletSrvc.SetClient(b.rpcClient.Ethclient())
		b.log.Info("SET REPC CLIETN")
		services = append(services, walletService)
	}

	b.log.Info("WALLET ENABLED")

	// We ignore for now local notifications flag as users who are upgrading have no mean to enable it
	services = append(services, b.localNotificationsService(config.NetworkID))

	b.log.Info("SET CLIENT")

	b.peerSrvc.SetDiscoverer(b)

	b.log.Info("SET DISCOVERER")

	for i := range services {
		b.gethNode.RegisterAPIs(services[i].APIs())
		b.gethNode.RegisterProtocols(services[i].Protocols())
		b.gethNode.RegisterLifecycle(services[i])
	}

	return nil
}

func (b *StatusNode) nodeBridge() types.Node {
	return gethbridge.NewNodeBridge(b.gethNode, b.wakuSrvc, b.wakuV2Srvc)
}

func (b *StatusNode) nodeBridgeService() *nodebridge.NodeService {
	if b.nodeBridgeSrvc == nil {
		b.nodeBridgeSrvc = &nodebridge.NodeService{Node: b.nodeBridge()}
	}
	return b.nodeBridgeSrvc
}

func (b *StatusNode) wakuExtService(config *params.NodeConfig) (*wakuext.Service, error) {
	if b.gethNode == nil {
		return nil, errors.New("geth node not initialized")
	}

	if b.wakuExtSrvc == nil {
		b.wakuExtSrvc = wakuext.New(config.ShhextConfig, b.nodeBridge(), ext.EnvelopeSignalHandler{}, b.db)
	}

	b.wakuExtSrvc.SetP2PServer(b.gethNode.Server())
	return b.wakuExtSrvc, nil
}

func (b *StatusNode) wakuV2ExtService(config *params.NodeConfig) (*wakuv2ext.Service, error) {
	if b.gethNode == nil {
		return nil, errors.New("geth node not initialized")
	}
	if b.wakuV2ExtSrvc == nil {
		b.wakuV2ExtSrvc = wakuv2ext.New(config.ShhextConfig, b.nodeBridge(), ext.EnvelopeSignalHandler{}, b.db)
	}

	b.wakuV2ExtSrvc.SetP2PServer(b.gethNode.Server())
	return b.wakuV2ExtSrvc, nil
}

func (b *StatusNode) WakuService() *waku.Waku {
	return b.wakuSrvc
}

func (b *StatusNode) WakuExtService() *wakuext.Service {
	return b.wakuExtSrvc
}

func (b *StatusNode) WakuV2ExtService() *wakuv2ext.Service {
	return b.wakuV2ExtSrvc
}
func (b *StatusNode) WakuV2Service() *wakuv2.Waku {
	return b.wakuV2Srvc
}

func (b *StatusNode) wakuService(wakuCfg *params.WakuConfig, clusterCfg *params.ClusterConfig) (*waku.Waku, error) {
	if b.wakuSrvc == nil {
		cfg := &waku.Config{
			MaxMessageSize:         wakucommon.DefaultMaxMessageSize,
			BloomFilterMode:        wakuCfg.BloomFilterMode,
			FullNode:               wakuCfg.FullNode,
			SoftBlacklistedPeerIDs: wakuCfg.SoftBlacklistedPeerIDs,
			MinimumAcceptedPoW:     params.WakuMinimumPoW,
			EnableConfirmations:    wakuCfg.EnableConfirmations,
		}

		if wakuCfg.MaxMessageSize > 0 {
			cfg.MaxMessageSize = wakuCfg.MaxMessageSize
		}
		if wakuCfg.MinimumPoW > 0 {
			cfg.MinimumAcceptedPoW = wakuCfg.MinimumPoW
		}

		w := waku.New(cfg, logutils.ZapLogger())

		if wakuCfg.EnableRateLimiter {
			r := wakuRateLimiter(wakuCfg, clusterCfg)
			w.RegisterRateLimiter(r)
		}

		if timesource := b.timeSource(); timesource != nil {
			w.SetTimeSource(timesource.Now)
		}

		// enable mail service
		if wakuCfg.EnableMailServer {
			if err := registerWakuMailServer(w, wakuCfg); err != nil {
				return nil, fmt.Errorf("failed to register WakuMailServer: %v", err)
			}
		}

		if wakuCfg.LightClient {
			emptyBloomFilter := make([]byte, 64)
			if err := w.SetBloomFilter(emptyBloomFilter); err != nil {
				return nil, err
			}
		}
		b.wakuSrvc = w
	}
	return b.wakuSrvc, nil

}

func (b *StatusNode) wakuV2Service(nodeKey string, wakuCfg *params.WakuV2Config, clusterCfg *params.ClusterConfig) (*wakuv2.Waku, error) {
	if b.wakuV2Srvc == nil {
		cfg := &wakuv2.Config{
			MaxMessageSize:         wakucommon.DefaultMaxMessageSize,
			SoftBlacklistedPeerIDs: wakuCfg.SoftBlacklistedPeerIDs,
			Host:                   wakuCfg.Host,
			Port:                   wakuCfg.Port,
			BootNodes:              clusterCfg.WakuNodes,
			StoreNodes:             clusterCfg.WakuStoreNodes,
		}

		if wakuCfg.MaxMessageSize > 0 {
			cfg.MaxMessageSize = wakuCfg.MaxMessageSize
		}

		lvl, err := logging.LevelFromString("info")
		if err != nil {
			panic(err)
		}
		logging.SetAllLoggers(lvl)

		w, err := wakuv2.New(nodeKey, cfg, logutils.ZapLogger())

		if err != nil {
			return nil, err
		}
		b.wakuV2Srvc = w
	}

	return b.wakuV2Srvc, nil
}

func wakuRateLimiter(wakuCfg *params.WakuConfig, clusterCfg *params.ClusterConfig) *wakucommon.PeerRateLimiter {
	enodes := append(
		parseNodes(clusterCfg.StaticNodes),
		parseNodes(clusterCfg.TrustedMailServers)...,
	)
	var (
		ips     []string
		peerIDs []enode.ID
	)
	for _, item := range enodes {
		ips = append(ips, item.IP().String())
		peerIDs = append(peerIDs, item.ID())
	}
	return wakucommon.NewPeerRateLimiter(
		&wakucommon.PeerRateLimiterConfig{
			PacketLimitPerSecIP:     wakuCfg.PacketRateLimitIP,
			PacketLimitPerSecPeerID: wakuCfg.PacketRateLimitPeerID,
			BytesLimitPerSecIP:      wakuCfg.BytesRateLimitIP,
			BytesLimitPerSecPeerID:  wakuCfg.BytesRateLimitPeerID,
			WhitelistedIPs:          ips,
			WhitelistedPeerIDs:      peerIDs,
		},
		&wakucommon.MetricsRateLimiterHandler{},
		&wakucommon.DropPeerRateLimiterHandler{
			Tolerance: wakuCfg.RateLimitTolerance,
		},
	)
}

func (b *StatusNode) rpcFiltersService() *rpcfilters.Service {
	if b.rpcFiltersSrvc == nil {
		b.rpcFiltersSrvc = rpcfilters.New(b)
	}
	return b.rpcFiltersSrvc
}

func (b *StatusNode) subscriptionService() *subscriptions.Service {
	if b.subscriptionsSrvc == nil {

		b.subscriptionsSrvc = subscriptions.New(func() *rpc.Client { return b.RPCPrivateClient() })
	}
	return b.subscriptionsSrvc
}

func (b *StatusNode) rpcStatsService() *rpcstats.Service {
	if b.rpcStatsSrvc == nil {
		b.rpcStatsSrvc = rpcstats.New()
	}

	return b.rpcStatsSrvc
}

func (b *StatusNode) accountsService(accountsFeed *event.Feed) *accountssvc.Service {
	if b.accountsSrvc == nil {
		b.accountsSrvc = accountssvc.NewService(accounts.NewDB(b.appDB), b.multiaccountsDB, b.gethAccountManager.Manager, accountsFeed)
	}

	return b.accountsSrvc
}

func (b *StatusNode) browsersService() *browsers.Service {
	if b.browsersSrvc == nil {
		b.browsersSrvc = browsers.NewService(browsers.NewDB(b.appDB))
	}
	return b.browsersSrvc
}

func (b *StatusNode) permissionsService() *permissions.Service {
	if b.permissionsSrvc == nil {
		b.permissionsSrvc = permissions.NewService(permissions.NewDB(b.appDB))
	}
	return b.permissionsSrvc
}

func (b *StatusNode) mailserversService() *mailservers.Service {
	if b.mailserversSrvc == nil {

		b.mailserversSrvc = mailservers.NewService(mailservers.NewDB(b.appDB))
	}
	return b.mailserversSrvc
}

func (b *StatusNode) appmetricsService() common.StatusService {
	if b.appMetricsSrvc == nil {
		b.appMetricsSrvc = appmetricsservice.NewService(appmetrics.NewDB(b.appDB))
	}
	return b.appMetricsSrvc
}

func (b *StatusNode) walletService(network uint64, accountsFeed *event.Feed) common.StatusService {
	if b.walletSrvc == nil {
		b.walletSrvc = wallet.NewService(wallet.NewDB(b.appDB, network), accountsFeed)
	}
	return b.walletSrvc
}

func (b *StatusNode) localNotificationsService(network uint64) *localnotifications.Service {
	if b.localNotificationsSrvc == nil {
		b.localNotificationsSrvc = localnotifications.NewService(b.appDB, network)
	}
	return b.localNotificationsSrvc
}

func (b *StatusNode) peerService() *peer.Service {
	if b.peerSrvc == nil {
		b.peerSrvc = peer.New()
	}
	return b.peerSrvc

}

func registerWakuMailServer(wakuService *waku.Waku, config *params.WakuConfig) (err error) {
	var mailServer mailserver.WakuMailServer
	wakuService.RegisterMailServer(&mailServer)

	return mailServer.Init(wakuService, config)
}

func appendIf(condition bool, services []common.StatusService, service common.StatusService) []common.StatusService {
	if !condition {
		return services
	}
	return append(services, service)
}

func (b *StatusNode) RPCFiltersService() *rpcfilters.Service {
	return b.rpcFiltersSrvc
}

func (b *StatusNode) StopLocalNotifications() error {
	if b.localNotificationsSrvc == nil {
		return nil
	}

	if b.localNotificationsSrvc.IsStarted() {
		err := b.localNotificationsSrvc.Stop()
		if err != nil {
			b.log.Error("LocalNotifications service stop failed on StopLocalNotifications", "error", err)
			return nil
		}
	}

	return nil
}

func (b *StatusNode) StartLocalNotifications() error {
	if b.localNotificationsSrvc == nil {
		return nil
	}

	if b.walletSrvc == nil {
		return nil
	}

	if !b.localNotificationsSrvc.IsStarted() {
		err := b.localNotificationsSrvc.Start()

		if err != nil {
			b.log.Error("LocalNotifications service start failed on StartLocalNotifications", "error", err)
			return nil
		}
	}

	err := b.localNotificationsSrvc.SubscribeWallet(b.walletSrvc.GetFeed())

	if err != nil {
		b.log.Error("LocalNotifications service could not subscribe to wallet on StartLocalNotifications", "error", err)
		return nil
	}

	return nil
}

// `personal_sign` and `personal_ecRecover` methods are important to
// keep DApps working.
// Usually, they are provided by an ETH or a LES service, but when using
// upstream, we don't start any of these, so we need to start our own
// implementation.

func (b *StatusNode) personalService() *personal.Service {
	if b.personalSrvc == nil {
		b.personalSrvc = personal.New(b.accountsManager)
	}
	return b.personalSrvc
}

func (b *StatusNode) timeSource() *timesource.NTPTimeSource {

	if b.timeSourceSrvc == nil {
		b.timeSourceSrvc = timesource.Default()
	}
	return b.timeSourceSrvc
}

func (b *StatusNode) Cleanup() error {
	if b.wakuSrvc != nil {
		if err := b.wakuSrvc.DeleteKeyPairs(); err != nil {
			return fmt.Errorf("%s: %v", ErrWakuClearIdentitiesFailure, err)
		}
	}

	if b.Config().WalletConfig.Enabled {
		if b.walletSrvc != nil {
			if b.walletSrvc.IsStarted() {
				err := b.walletSrvc.Stop()
				if err != nil {
					return err
				}
			}
		}
	}
	return nil

}