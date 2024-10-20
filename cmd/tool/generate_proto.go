// generate_proto.go 构建proto相关内容

//go:generate go run generate_proto.go all
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	protoApiDirectory = "./proto/api"
)

func ensureDir(dirName string) error {
	_, err := os.Stat(dirName)

	if os.IsNotExist(err) {
		return os.MkdirAll(dirName, 0755)
	}
	return err
}

func findProto(directory string) []string {
	var protoFiles []string
	filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".proto") {
			relativePath, _ := filepath.Rel(directory, path)
			protoFiles = append(protoFiles, filepath.Join(directory, relativePath))
		}
		return nil
	})
	return protoFiles
}

func generateProtoAPI() {
	if err := ensureDir("./api"); err != nil {
		panic(err)
	}
	protoFiles := findProto(protoApiDirectory)
	if len(protoFiles) <= 0 {
		return
	}
	args := []string{
		"--proto_path=./proto/api",
		"--proto_path=./proto/extension",
		"--go_out=paths=source_relative:./api",
		"--go-http_out=paths=source_relative:./api",
		"--go-grpc_out=paths=source_relative:./api",
	}
	args = append(args, protoFiles...)
	cmd := exec.Command("protoc", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("protoc %s\n", strings.Join(args, " "))
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}

func generateProtoConfig() {
	args := []string{
		"--proto_path=./conf",
		"--proto_path=./proto/extension",
		"--go_out=paths=source_relative:./conf",
		"./conf/conf.proto",
	}
	cmd := exec.Command("protoc", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("protoc %s\n", strings.Join(args, " "))
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}

func generateProtoOpenAPI() {
	protoFiles := findProto(protoApiDirectory)
	args := []string{
		"--proto_path=./proto/api",
		"--proto_path=./proto/extension",
		"--openapi_out=fq_schema_naming=true,default_response=false:.",
	}
	args = append(args, protoFiles...)
	cmd := exec.Command("protoc", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("protoc %s\n", strings.Join(args, " "))
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}

func main() {
	pwd, _ := os.Getwd()
	pwd = filepath.Join(pwd, "../..")
	err := os.Chdir(pwd)
	if err != nil {
		panic(err)
	}

	arguments := os.Args[1:]
	command := "all"
	if len(arguments) > 0 {
		command, arguments = arguments[0], arguments[1:]
	}
	_ = arguments

	switch command {
	case "api":
		generateProtoAPI()
	case "conf":
		generateProtoConfig()
	case "openapi":
		generateProtoOpenAPI()
	case "all":
		generateProtoAPI()
		generateProtoConfig()
		generateProtoOpenAPI()
	}
}
