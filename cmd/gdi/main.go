package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sjqzhang/gdi/pkg/processor"
)

var (
	debug   = os.Getenv("GDI_DEBUG") == "1"
	logFile = os.Getenv("GDI_LOG")
)

func debugf(format string, args ...interface{}) {
	if debug {
		msg := fmt.Sprintf("[GDI_DEBUG] "+format+"\n", args...)
		fmt.Print(msg)
		if logFile != "" {
			f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				defer f.Close()
				f.WriteString(msg)
			}
		}
	}
}

func main() {
	pwd, _ := os.Getwd()
	debugf("当前工作目录: %s", pwd)
	debugf("GOPATH: %s", os.Getenv("GOPATH"))
	debugf("工具链启动，参数: %v", os.Args)

	if len(os.Args) < 2 {
		fmt.Println("Usage: gdi <go tool> [args...]")
		os.Exit(1)
	}

	// 检查是否是编译命令
	isCompile := strings.Contains(os.Args[1], "compile")
	if !isCompile {
		debugf("非编译命令，直接执行原始工具")
		executeOriginalTool()
		return
	}

	// 如果是版本检查命令，直接执行
	if len(os.Args) > 2 && strings.HasPrefix(os.Args[2], "-V") {
		debugf("版本检查命令，直接执行")
		executeOriginalTool()
		return
	}

	// 查找源文件参数
	var sourceFile string
	for _, arg := range os.Args[2:] {
		if strings.HasSuffix(arg, ".go") && !strings.HasPrefix(arg, "-") {
			// 确保使用绝对路径
			if !filepath.IsAbs(arg) {
				if abs, err := filepath.Abs(arg); err == nil {
					sourceFile = abs
				} else {
					sourceFile = arg
				}
			} else {
				sourceFile = arg
			}
			debugf("找到源文件: %s", sourceFile)
			break
		}
	}

	if sourceFile == "" {
		debugf("未找到源文件，直接执行原始工具")
		executeOriginalTool()
		return
	}

	// 确保源文件存在
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		debugf("源文件不存在: %s", sourceFile)
		executeOriginalTool()
		return
	}

	debugf("处理源文件: %s", sourceFile)

	// 创建调试目录
	debugDir := ""
	if debug {
		debugDir = filepath.Join(os.TempDir(), "gdi_debug")
		os.MkdirAll(debugDir, 0755)
		debugf("调试目录: %s", debugDir)
	}

	// 处理源文件
	debugf("开始处理源文件: %s", sourceFile)
	debugf("调试目录: %s", debugDir)
	processedFile, err := processor.ProcessFile(sourceFile, debugDir)
	if err != nil {
		debugf("处理文件失败: %v", err)
		executeOriginalTool()
		return
	}

	// 如果文件被处理，替换参数
	if processedFile != sourceFile {
		debugf("文件已处理，新位置: %s", processedFile)
		// 替换参数中的源文件
		for i, arg := range os.Args {
			if arg == sourceFile {
				os.Args[i] = processedFile
				break
			}
		}
	}

	debugf("执行编译命令: %v", os.Args)
	executeOriginalTool()
}

func executeOriginalTool() {
	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin // 添加这行
	if err := cmd.Run(); err != nil {
		debugf("工具执行失败: %v", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
