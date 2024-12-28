package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sjqzhang/gdi/pkg/processor"
)

var (
	debug   = os.Getenv("GDI_DEBUG") == "1"
	logFile = os.Getenv("GDI_LOG")
)

func debugf(format string, args ...interface{}) {
	if debug {
		// 获取调用者的文件和行号
		_, file, line, _ := runtime.Caller(1)
		// 只取文件名，不要完整路径
		file = filepath.Base(file)

		msg := fmt.Sprintf("[GDI_DEBUG][%s:%d] "+format+"\n", append([]interface{}{file, line}, args...)...)
		// 将调试信息写入标准错误，而不是标准输出
		fmt.Fprint(os.Stderr, msg)
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
		fmt.Fprintln(os.Stderr, "Usage: gdi <go tool> [args...]")
		os.Exit(1)
	}

	// 检查是否是编译命令
	isCompile := strings.Contains(os.Args[1], "compile")
	debugf("是否是编译命令: %v", isCompile)
	if !isCompile {
		debugf("非编译命令，直接执行原始工具")
		executeOriginalTool()
		return
	}

	debugf("参数长度: %d, 参数列表: %v", len(os.Args), os.Args)
	// 如果只是版本检查命令，直接执行并返回输出
	if len(os.Args) == 3 && strings.HasPrefix(os.Args[2], "-V") {
		debugf("版本检查命令，直接执行并返回输出")
		cmd := exec.Command(os.Args[1], os.Args[2:]...)
		cmd.Stderr = os.Stderr
		output, err := cmd.Output()
		if err != nil {
			debugf("版本检查失败: %v", err)
			os.Exit(1)
		}
		// 确保版本信息直接写入标准输出，不包含任何调试信息
		os.Stdout.Write(output)
		return
	}

	// 查找源文件参数
	var sourceFile string
	debugf("开始查找源文件，参数列表: %v", os.Args[2:])
	for i, arg := range os.Args[2:] {
		debugf("检查第 %d 个参数: %s", i+1, arg)
		if strings.HasSuffix(arg, ".go") && !strings.HasPrefix(arg, "-") {
			// 确保使用绝对路径
			if !filepath.IsAbs(arg) {
				abs, err := filepath.Abs(arg)
				if err != nil {
					debugf("转换绝对路径失败: %v", err)
					sourceFile = arg
				} else {
					sourceFile = abs
				}
			} else {
				sourceFile = arg
			}
			debugf("找到源文件: %s", sourceFile)
			break
		}
	}

	if sourceFile == "" {
		debugf("未找到源文件，参数中没有.go文件")
		executeOriginalTool()
		return
	}

	// 确保源文件存在
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		debugf("源文件不存在: %s, 错误: %v", sourceFile, err)
		executeOriginalTool()
		return
	}

	debugf("开始处理源文件: %s", sourceFile)

	// 创建调试目录
	debugDir := ""
	if debug {
		debugDir = filepath.Join(os.TempDir(), "gdi_debug")
		if err := os.MkdirAll(debugDir, 0755); err != nil {
			debugf("创建调试���录失败: %v", err)
		}
		debugf("创建调试目录成功: %s", debugDir)
	}

	// 处理源文件
	debugf("开始调用 ProcessFile 处理源文件: %s", sourceFile)
	processedFile, err := processor.ProcessFile(sourceFile, debugDir)
	if err != nil {
		debugf("ProcessFile 处理失败: %v", err)
		debugf("使用原始文件继续编译: %s", sourceFile)
		executeOriginalTool()
		return
	}

	debugf("ProcessFile 处理完成，处理后的文件: %s", processedFile)

	// 如果文件被处理替换参数
	if processedFile != sourceFile {
		debugf("需要替换源文件参数，从 %s 到 %s", sourceFile, processedFile)
		// 替换参数中的源文件
		found := false
		for i, arg := range os.Args {
			if arg == sourceFile {
				debugf("在参数位置 %d 找到源文件，进行替换", i)
				os.Args[i] = processedFile
				found = true
				break
			}
		}
		if !found {
			debugf("警告：在参数列表中未找到源文件路径，这可能会导致编译问题")
		}
	} else {
		debugf("处理后的文件与源文件相同，不需要替换参数")
	}

	debugf("准备执行编译命令，完整参数: %v", os.Args)
	executeOriginalTool()
}

func executeOriginalTool() {
	debugf("开始执行原始工具")
	debugf("工具路径: %s", os.Args[1])
	debugf("工具参数: %v", os.Args[2:])

	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// 记录命令输出到日志
	if logFile != "" && !strings.HasPrefix(os.Args[2], "-V") {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			debugf("成功打开日志文件: %s", logFile)
			defer f.Close()
			cmd.Stdout = io.MultiWriter(os.Stdout, f)
			cmd.Stderr = io.MultiWriter(os.Stderr, f)
		} else {
			debugf("打开日志文件失败: %v", err)
		}
	}

	debugf("开始执行命令")
	if err := cmd.Run(); err != nil {
		debugf("命令执行失败: %v", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
	debugf("命令执行成功完成")
}
