package data

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/nftables"
	col "github.com/noxiouz/golang-generics-util/collection"
	"net"
	"time"
	"transmission-proxy/internal/domain"
)

type banIPDao struct {
	infra *Infra
	log   *log.Helper

	// banlistIPV4 IPV4黑名单列表
	banlistIPV4 map[string]time.Time

	// banlistIPV6 IPV6黑名单列表
	banlistIPV6 map[string]time.Time
}

// NewBanIPDao .
func NewBanIPDao(infra *Infra, logger log.Logger) domain.BanIPRepo {
	return &banIPDao{
		infra: infra,
		log:   log.NewHelper(logger),

		banlistIPV4: make(map[string]time.Time, 1000),
		banlistIPV6: make(map[string]time.Time, 1000),
	}
}

// GetBannedIPV4Status 获取封禁ipv4状态
func (d *banIPDao) GetBannedIPV4Status(_ context.Context, ips []string) (map[string]col.Option[*time.Time], error) {
	statuses := make(map[string]col.Option[*time.Time], len(ips))
	for _, ip := range ips {
		banTime, ok := d.banlistIPV4[ip]
		if ok {
			statuses[ip] = col.Some(&banTime)
		} else {
			statuses[ip] = col.None[*time.Time]()
		}
	}
	return statuses, nil
}

// GetBannedIPV6Status 获取封禁ipv6状态
func (d *banIPDao) GetBannedIPV6Status(_ context.Context, ips []string) (map[string]col.Option[*time.Time], error) {
	statuses := make(map[string]col.Option[*time.Time], len(ips))
	for _, ip := range ips {
		banTime, ok := d.banlistIPV6[ip]
		if ok {
			statuses[ip] = col.Some(&banTime)
		} else {
			statuses[ip] = col.None[*time.Time]()
		}
	}
	return statuses, nil
}

// BanIPV4 封禁ipv4
func (d *banIPDao) BanIPV4(_ context.Context, ips []string) error {
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

	nowTime := time.Now()
	for _, netIP := range readyIP {
		d.banlistIPV4[netIP.String()] = nowTime
	}
	return nil
}

// BanIPV6 封禁ipv6
func (d *banIPDao) BanIPV6(_ context.Context, ips []string) error {
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

	nowTime := time.Now()
	for _, netIP := range readyIP {
		d.banlistIPV4[netIP.String()] = nowTime
	}
	return nil
}

// UnbanIPV4 解禁ipv4
func (d *banIPDao) UnbanIPV4(_ context.Context, ips []string) error {
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
		d.banlistIPV4[netIP.String()] = time.Now()
	}
	return nil
}

// UnbanIPV6 解禁ipv6
func (d *banIPDao) UnbanIPV6(_ context.Context, ips []string) error {
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
		d.banlistIPV4[netIP.String()] = time.Now()
	}
	return nil
}

func (d *banIPDao) UpBanIPV4List(ctx context.Context, ips []string) (err error) {
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

func (d *banIPDao) UpBanIPV6List(ctx context.Context, ips []string) (err error) {
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
func (d *banIPDao) ClearBanList(_ context.Context) (err error) {
	d.banlistIPV4 = make(map[string]time.Time, len(d.banlistIPV4))
	d.banlistIPV6 = make(map[string]time.Time, len(d.banlistIPV6))
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
