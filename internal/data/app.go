package data

import (
	"context"
	"net"

	"transmission-proxy/internal/domain"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/nftables"
	"github.com/hekmon/transmissionrpc/v3"
)

var preferencesFields = []string{
	"start-added-torrents",
	"rename-partial-files",
	"download-dir",
	"incomplete-dir-enabled",
	"incomplete-dir",
	"script-torrent-done-enabled",
	"script-torrent-done-filename",
	"download-queue-enabled",
	"download-queue-size",
	"seed-queue-enabled",
	"seed-queue-size",
	"seedRatioLimited",
	"seedRatioLimit",
	"peer-port",
	"peer-port-random-on-start",
	"alt-speed-down",
	"alt-speed-enabled",
	"alt-speed-up",
	"peer-limit-global",
	"peer-limit-per-torrent",
	"version",
}

type appDao struct {
	infra *Infra
	log   *log.Helper

	// banlistIPV4 IPV4黑名单列表
	banlistIPV4 map[string]struct{}

	// banlistIPV6 IPV6黑名单列表
	banlistIPV6 map[string]struct{}
}

// NewAppDao .
func NewAppDao(infra *Infra, logger log.Logger) domain.AppRepo {
	return &appDao{
		infra: infra,
		log:   log.NewHelper(logger),

		banlistIPV4: make(map[string]struct{}, 1000),
		banlistIPV6: make(map[string]struct{}, 1000),
	}
}

// GetBannedIPV4Status 获取封禁ipv4状态
func (d *appDao) GetBannedIPV4Status(_ context.Context, ips []string) (map[string]bool, error) {
	statuses := make(map[string]bool, len(ips))
	for _, ip := range ips {
		_, ok := d.banlistIPV4[ip]
		statuses[ip] = ok
	}
	return statuses, nil
}

// GetBannedIPV6Status 获取封禁ipv6状态
func (d *appDao) GetBannedIPV6Status(_ context.Context, ips []string) (map[string]bool, error) {
	statuses := make(map[string]bool, len(ips))
	for _, ip := range ips {
		_, ok := d.banlistIPV6[ip]
		statuses[ip] = ok
	}
	return statuses, nil
}

// BanIPV4 封禁ipv4
func (d *appDao) BanIPV4(_ context.Context, ips []string) error {
	readyIP := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		netIP := net.ParseIP(ip)
		if netIP != nil {
			readyIP = append(readyIP, netIP)
		}
	}

	elements := make([]nftables.SetElement, 0, len(readyIP))
	for _, netIP := range readyIP {
		elements = append(elements, nftables.SetElement{Key: netIP.To4()})
	}
	err := d.infra.NFT.SetAddElements(BanIPV4Set, elements)
	if err != nil {
		return err
	}
	if err := d.infra.NFT.Flush(); err != nil {
		return err
	}

	for _, netIP := range readyIP {
		d.banlistIPV4[netIP.String()] = struct{}{}
	}
	return nil
}

// BanIPV6 封禁ipv6
func (d *appDao) BanIPV6(_ context.Context, ips []string) error {
	readyIP := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		ipNet := net.ParseIP(ip)
		if ipNet != nil {
			readyIP = append(readyIP, ipNet)
		}
	}

	elements := make([]nftables.SetElement, 0, len(readyIP))
	for _, netIP := range readyIP {
		elements = append(elements, nftables.SetElement{Key: netIP.To16()})
	}
	err := d.infra.NFT.SetAddElements(BanIPV6Set, elements)
	if err != nil {
		return err
	}
	if err := d.infra.NFT.Flush(); err != nil {
		return err
	}

	for _, netIP := range readyIP {
		d.banlistIPV4[netIP.String()] = struct{}{}
	}
	return nil
}

// UnbanIPV4 解禁ipv4
func (d *appDao) UnbanIPV4(ctx context.Context, ips []string) error {
	readyIP := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		netIP := net.ParseIP(ip)
		if netIP != nil {
			readyIP = append(readyIP, netIP)
		}
	}

	elements := make([]nftables.SetElement, 0, len(readyIP))
	for _, netIP := range readyIP {
		elements = append(elements, nftables.SetElement{Key: netIP.To4()})
	}
	err := d.infra.NFT.SetDeleteElements(BanIPV4Set, elements)
	if err != nil {
		return err
	}
	if err := d.infra.NFT.Flush(); err != nil {
		return err
	}

	for _, netIP := range readyIP {
		d.banlistIPV4[netIP.String()] = struct{}{}
	}
	return nil
}

// UnbanIPV6 解禁ipv6
func (d *appDao) UnbanIPV6(ctx context.Context, ips []string) error {
	readyIP := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		ipNet := net.ParseIP(ip)
		if ipNet != nil {
			readyIP = append(readyIP, ipNet)
		}
	}

	elements := make([]nftables.SetElement, 0, len(readyIP))
	for _, netIP := range readyIP {
		elements = append(elements, nftables.SetElement{Key: netIP.To16()})
	}
	err := d.infra.NFT.SetDeleteElements(BanIPV6Set, elements)
	if err != nil {
		return err
	}
	if err := d.infra.NFT.Flush(); err != nil {
		return err
	}

	for _, netIP := range readyIP {
		d.banlistIPV4[netIP.String()] = struct{}{}
	}
	return nil
}

func (d *appDao) UpBanIPV4List(ctx context.Context, ips []string) (err error) {
	ipSet := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		ipSet[ip] = struct{}{}
	}
	readyAdd := make([]string, 0, len(ips))
	readyRemove := make([]string, 0, len(d.banlistIPV4))
	for _, ip := range ips {
		_, exist := d.banlistIPV4[ip]
		if !exist {
			readyAdd = append(readyAdd, ip)
		}
	}
	for ip, _ := range d.banlistIPV4 {
		_, exist := ipSet[ip]
		if !exist {
			readyRemove = append(readyRemove, ip)
		}
	}
	err = d.BanIPV4(ctx, readyAdd)
	if err != nil {
		return
	}
	err = d.UnbanIPV4(ctx, readyRemove)
	if err != nil {
		return err
	}
	return
}

func (d *appDao) UpBanIPV6List(ctx context.Context, ips []string) (err error) {
	ipSet := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		ipSet[ip] = struct{}{}
	}
	readyAdd := make([]string, 0, len(ips))
	readyRemove := make([]string, 0, len(d.banlistIPV6))
	for _, ip := range ips {
		_, exist := d.banlistIPV6[ip]
		if !exist {
			readyAdd = append(readyAdd, ip)
		}
	}
	for ip, _ := range d.banlistIPV6 {
		_, exist := ipSet[ip]
		if !exist {
			readyRemove = append(readyRemove, ip)
		}
	}
	err = d.BanIPV4(ctx, readyAdd)
	if err != nil {
		return
	}
	err = d.UnbanIPV4(ctx, readyRemove)
	if err != nil {
		return err
	}
	return
}

// ClearBanList 清空Ban列表
func (d *appDao) ClearBanList(_ context.Context) (err error) {
	d.banlistIPV4 = make(map[string]struct{}, len(d.banlistIPV4))
	d.banlistIPV6 = make(map[string]struct{}, len(d.banlistIPV6))
	// 重置 set
	d.infra.NFT.DelSet(BanIPV4Set)
	d.infra.NFT.DelSet(BanIPV6Set)
	err = d.infra.NFT.AddSet(BanIPV4Set, nil)
	if err != nil {
		return
	}
	err = d.infra.NFT.AddSet(BanIPV6Set, nil)
	if err != nil {
		return
	}
	err = d.infra.NFT.Flush()
	return
}

// GetPreferences 获取首选项
func (d *appDao) GetPreferences(ctx context.Context) (transmissionrpc.SessionArguments, error) {
	pre, err := d.infra.TR.SessionArgumentsGet(ctx, preferencesFields)
	if err != nil {
		return transmissionrpc.SessionArguments{}, err
	}
	return pre, nil
}

// SetPreferences 设置首选项
func (d *appDao) SetPreferences(ctx context.Context, pre transmissionrpc.SessionArguments) error {
	err := d.infra.TR.SessionArgumentsSet(ctx, pre)
	if err != nil {
		return err
	}
	return nil
}
