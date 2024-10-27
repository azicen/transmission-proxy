package domain

import (
	"context"
	"net"

	pb "transmission-proxy/api/v2"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/hekmon/transmissionrpc/v3"
	col "github.com/noxiouz/golang-generics-util/collection"
)

// AppRepo .
type AppRepo interface {
	// GetPreferences 获取首选项
	GetPreferences(ctx context.Context) (transmissionrpc.SessionArguments, error)

	// SetPreferences 设置首选项
	SetPreferences(ctx context.Context, trd transmissionrpc.SessionArguments) error
}

type Preferences struct {
	ListenPort col.Option[int32]
	BanList    col.Option[[]string]
}

// AppUsecase .
type AppUsecase struct {
	appRepo   AppRepo
	banIPRepo BanIPRepo
	log       *log.Helper
}

// NewAppUsecase .
func NewAppUsecase(appRepo AppRepo, banIPRepo BanIPRepo, logger log.Logger) *AppUsecase {

	return &AppUsecase{
		appRepo:   appRepo,
		banIPRepo: banIPRepo,
		log:       log.NewHelper(logger),
	}
}

// BanIP 封禁IP
func (uc *AppUsecase) BanIP(ctx context.Context, ips []string) error {
	// 过滤错误的ip
	readyIPV4 := make([]string, 0, len(ips))
	readyIPV6 := make([]string, 0, len(ips))
	for _, ip := range ips {
		ipNet := net.ParseIP(ip)
		if ipNet == nil {
			continue
		}
		if ipNet.To4() != nil {
			readyIPV4 = append(readyIPV4, ipNet.String())
		} else {
			readyIPV6 = append(readyIPV6, ipNet.String())
		}
	}

	// 过滤已经封禁的ip
	ipStatuses, err := uc.banIPRepo.GetBannedIPV4Status(ctx, readyIPV4)
	if err != nil {
		return err
	}
	readyIPV4 = make([]string, 0, len(ipStatuses))
	for ip, banTime := range ipStatuses {
		if !banTime.HasValue() {
			readyIPV4 = append(readyIPV4, ip)
		}
	}
	ipStatuses, err = uc.banIPRepo.GetBannedIPV6Status(ctx, readyIPV6)
	if err != nil {
		return err
	}
	readyIPV6 = make([]string, 0, len(ipStatuses))
	for ip, banTime := range ipStatuses {
		if !banTime.HasValue() {
			readyIPV6 = append(readyIPV6, ip)
		}
	}

	// 封禁
	if len(readyIPV4) != 0 {
		err := uc.banIPRepo.BanIPV4(ctx, readyIPV4)
		if err != nil {
			return err
		}
	}
	if len(readyIPV6) != 0 {
		err := uc.banIPRepo.BanIPV6(ctx, readyIPV6)
		if err != nil {
			return err
		}
	}
	return nil
}

// UnbanIP 解禁IP
func (uc *AppUsecase) UnbanIP(ctx context.Context, ips []string) error {
	// 过滤错误的ip
	readyIPV4 := make([]string, 0, len(ips))
	readyIPV6 := make([]string, 0, len(ips))
	for _, ip := range ips {
		ipNet := net.ParseIP(ip)
		if ipNet == nil {
			continue
		}
		if ipNet.To4() != nil {
			readyIPV4 = append(readyIPV4, ipNet.String())
		} else {
			readyIPV6 = append(readyIPV6, ipNet.String())
		}
	}

	// 过滤没有封禁的ip
	ipStatuses, err := uc.banIPRepo.GetBannedIPV4Status(ctx, readyIPV4)
	if err != nil {
		return err
	}
	readyIPV4 = make([]string, 0, len(ipStatuses))
	for ip, banTime := range ipStatuses {
		if banTime.HasValue() {
			readyIPV4 = append(readyIPV4, ip)
		}
	}
	ipStatuses, err = uc.banIPRepo.GetBannedIPV6Status(ctx, readyIPV6)
	if err != nil {
		return err
	}
	readyIPV6 = make([]string, 0, len(ipStatuses))
	for ip, banTime := range ipStatuses {
		if banTime.HasValue() {
			readyIPV6 = append(readyIPV6, ip)
		}
	}

	// 解禁
	if len(readyIPV4) != 0 {
		err := uc.banIPRepo.UnbanIPV4(ctx, readyIPV4)
		if err != nil {
			return err
		}
	}
	if len(readyIPV6) != 0 {
		err := uc.banIPRepo.UnbanIPV6(ctx, readyIPV6)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpBanIPList 完全更新IP列表
func (uc *AppUsecase) UpBanIPList(ctx context.Context, ips []string) (err error) {
	// 过滤错误的ip
	readyIPV4 := make([]string, 0, len(ips))
	readyIPV6 := make([]string, 0, len(ips))
	for _, ip := range ips {
		ipNet := net.ParseIP(ip)
		if ipNet == nil {
			continue
		}
		if ipNet.To4() != nil {
			readyIPV4 = append(readyIPV4, ipNet.String())
		} else {
			readyIPV6 = append(readyIPV6, ipNet.String())
		}
	}

	// 清空ban列表
	err = uc.banIPRepo.ClearBanList(ctx)
	if err != nil {
		return
	}

	// 全量ban
	err = uc.banIPRepo.BanIPV4(ctx, readyIPV4)
	if err != nil {
		return
	}
	err = uc.banIPRepo.BanIPV6(ctx, readyIPV6)
	if err != nil {
		return
	}
	return
}

func (uc *AppUsecase) GetPreferences(ctx context.Context) (*pb.GetPreferencesResponse, error) {
	pre, err := uc.appRepo.GetPreferences(ctx)
	if err != nil {
		return nil, err
	}

	maxActiveDownloads := int32(-1) // 最大同时下载数
	if *pre.DownloadQueueEnabled && pre.DownloadQueueSize != nil {
		maxActiveDownloads = int32(*pre.DownloadQueueSize)
	}
	maxActiveUploads := int32(-1) // 最大同时上传数
	if *pre.SeedQueueEnabled && pre.SeedQueueSize != nil {
		maxActiveUploads = int32(*pre.SeedQueueSize)
	}
	maxActiveTorrents := int32(-1) // 最大同时下载和上传数
	if *pre.DownloadQueueEnabled || *pre.SeedQueueEnabled {
		maxActiveTorrents = int32(0)
	}
	if maxActiveDownloads > 0 {
		maxActiveTorrents = maxActiveTorrents + maxActiveDownloads
	}
	if maxActiveUploads > 0 {
		maxActiveTorrents = maxActiveTorrents + maxActiveUploads
	}

	dlLimit := int32(-1)
	upLimit := int32(-1)
	if *pre.AltSpeedEnabled && pre.AltSpeedDown != nil {
		dlLimit = int32(*pre.AltSpeedDown)
	}
	if *pre.AltSpeedEnabled && pre.AltSpeedUp != nil {
		upLimit = int32(*pre.AltSpeedUp)
	}

	qbd := &pb.GetPreferencesResponse{
		Locale:                    "en_GB",
		CreateSubfolderEnabled:    false,                    // 添加种子时是否创建子文件夹
		StartPausedEnabled:        !*pre.StartAddedTorrents, // 种子是否以暂停状态添加
		AutoDeleteMode:            0,                        // 自动删除模式
		PreallocateAll:            false,                    // 是否为所有文件预分配磁盘空间
		IncompleteFilesExt:        *pre.RenamePartialFiles,  // 是否为未完成的文件添加".!qB"后缀
		AutoTmmEnabled:            false,                    // 是否默认启用自动种子管理
		TorrentChangedTmmEnabled:  false,                    // 当分类改变时是否重新定位种子
		SavePathChangedTmmEnabled: false,                    // 当默认保存路径更改时是否重新定位种子
		CategoryChangedTmmEnabled: false,                    // 当分类的保存路径改变时是否重新定位种子

		SavePath:        *pre.DownloadDir,          // 种子的默认保存路径
		TempPathEnabled: *pre.IncompleteDirEnabled, // 是否启用未完成种子的临时文件夹
		TempPath:        *pre.IncompleteDir,        // 未完成种子的临时文件夹路径
		ScanDirs:        make(map[string]string),   // 监控目录及其下载路径映射
		ExportDir:       "",                        // 将 .torrent 文件复制到的目录路径
		ExportDirFin:    "",                        // 将完成下载的 .torrent 文件复制到的目录路径

		MailNotificationEnabled:     false, // 是否启用电子邮件通知
		MailNotificationSender:      "",    // 发送通知的电子邮件地址
		MailNotificationEmail:       "",    // 要发送通知的电子邮件地址
		MailNotificationSmtp:        "",    // SMTP 服务器地址
		MailNotificationSslEnabled:  false, // SMTP 服务器是否需要 SSL 连接
		MailNotificationAuthEnabled: false, // SMTP 服务器是否需要认证
		MailNotificationUsername:    "",    // SMTP 认证用户名
		MailNotificationPassword:    "",    // SMTP 认证密码

		AutorunEnabled: *pre.ScriptTorrentDoneEnabled,  // 种子下载完成后是否运行外部程序
		AutorunProgram: *pre.ScriptTorrentDoneFilename, // 如果启用了 autorun_enabled，要运行的程序路径、名称和参数

		QueueingEnabled:    false,              // 是否启用种子队列
		MaxActiveDownloads: maxActiveDownloads, // 最大同时下载数
		MaxActiveTorrents:  maxActiveTorrents,  // 最大同时下载和上传数
		MaxActiveUploads:   maxActiveUploads,   // 最大同时上传数

		DontCountSlowTorrents:      false, // 是否将无活动的种子排除在限制之外
		SlowTorrentDlRateThreshold: 0,     // 认为种子下载速度“慢”的阈值
		SlowTorrentUlRateThreshold: 0,     // 认为种子上传速度“慢”的阈值
		SlowTorrentInactiveTimer:   0,     // 种子被认为“慢”之前的无活动时间

		MaxRatioEnabled: *pre.SeedRatioLimited,        // 是否启用分享率限制
		MaxRatio:        float32(*pre.SeedRatioLimit), // 全局分享率限制
		MaxRatioAct:     0,                            // 达到分享率限制后的动作 0 暂停激流; 1 删除激流
		ListenPort:      int32(*pre.PeerPort),         // 用于传入连接的端口
		Upnp:            false,                        // 是否启用 UPnP/NAT-PMP
		RandomPort:      *pre.PeerPortRandomOnStart,   // 是否随机选择端口

		DlLimit:              dlLimit,                         // 全局下载速度限制
		UpLimit:              upLimit,                         // 全局上传速度限制
		MaxConnec:            int32(*pre.PeerLimitGlobal),     // 最大全局连接数
		MaxConnecPerTorrent:  int32(*pre.PeerLimitPerTorrent), // 每个种子的最大连接数
		MaxUploads:           -1,                              // 最大上传数
		MaxUploadsPerTorrent: -1,                              // 每个种子的最大上传数
	}
	return qbd, nil
}

func (uc *AppUsecase) SetPreferences(ctx context.Context, pre *Preferences) (err error) {
	if pre.ListenPort.HasValue() {
		peerPort := int64(pre.ListenPort.Value())
		trd := transmissionrpc.SessionArguments{
			PeerPort: &peerPort,
		}
		err = uc.appRepo.SetPreferences(ctx, trd)
		if err != nil {
			return
		}
	}

	if pre.BanList.HasValue() {
		err = uc.BanIP(ctx, pre.BanList.Value())
		if err != nil {
			return
		}
	}
	return
}
