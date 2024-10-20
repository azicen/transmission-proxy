package conf

import (
	"embed"
	"os"
	"path/filepath"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/env"
	"github.com/go-kratos/kratos/v2/config/file"
)

const (
	TemplateName = "conf.template.toml"
)

//go:embed conf.template.toml
var TemplateFS embed.FS

// LoadConf 读取配置文件
func LoadConf(dir string, s ...config.Source) (*Bootstrap, func(), error) {
	confFile := filepath.Join(dir, "conf.toml")
	// 检查文件是否存在
	if _, err := os.Stat(confFile); os.IsNotExist(err) {
		err := CopyFS(TemplateFS, TemplateName, confFile)
		if err != nil {
			return nil, func() {}, err
		}
	}

	source := []config.Source{
		env.NewSource("TRP_"),
		file.NewSource(dir),
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
