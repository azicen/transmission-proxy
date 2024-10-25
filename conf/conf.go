package conf

import (
	"embed"
	"flag"
	"os"
	"path/filepath"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/env"
	"github.com/go-kratos/kratos/v2/config/file"
)

var (
	// FlagConf 配置文件目录
	FlagConf string
)

func init() {
	flag.StringVar(&FlagConf, "conf", "./conf", "config path, eg: -conf conf.toml")
}

const (
	ENVPrefix      = "TRP_"
	TemplateName   = "conf.template.toml"
	ConfigFileName = "conf.toml"
)

//go:embed conf.template.toml
var TemplateFS embed.FS

// LoadConf 读取配置文件
func LoadConf(path string, s ...config.Source) (*Bootstrap, func(), error) {
	path = filepath.Join(path, ConfigFileName)
	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := CopyFS(TemplateFS, TemplateName, path)
		if err != nil {
			return nil, func() {}, err
		}
	}

	source := []config.Source{
		env.NewSource(ENVPrefix),
		file.NewSource(path),
	}
	source = append(source, s...)
	c := config.New(
		config.WithSource(source...),
	)
	if err := c.Load(); err != nil {
		return nil, func() {}, err
	}
	bc := &Bootstrap{}
	if err := c.Scan(bc); err != nil {
		return nil, func() {}, err
	}

	cleanup := func() {
		c.Close()
	}

	return bc, cleanup, nil
}

// CopyFS 从嵌入文件中复
func CopyFS(fs embed.FS, fsName string, dest string) error {
	dir := filepath.Dir(dest)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}

	// 从嵌入的文件系统中提取文件
	data, err := fs.ReadFile(fsName)
	if err != nil {
		return err
	}

	// 将文件写入目标路径
	err = os.WriteFile(dest, data, 0644)
	if err != nil {
		return err
	}
	return nil
}
