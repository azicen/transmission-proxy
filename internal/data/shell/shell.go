package shell

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ExecCommand 执行命令
func ExecCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cmd=%s %s, cmd-out=%v, cmd-error=%v, ",
			name, strings.Join(args, " "), string(output), err)
	}
	return nil
}

// ExecCommandOutput 执行命令并且返回输出
func ExecCommandOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	res := string(output)
	if err != nil {
		return "", fmt.Errorf("cmd=%s %s, cmd-out=%v, cmd-error=%v",
			name, strings.Join(args, " "), res, err)
	}
	return res, nil
}

// ExecCommandStdout 执行命令并且返回输出到终端
func ExecCommandStdout(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("cmd=%s %s, cmd-error=%v",
			name, strings.Join(args, " "), err)
	}
	return nil
}
