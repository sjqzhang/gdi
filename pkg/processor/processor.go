package processor

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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

// ProcessFile 处理单个源文件
func ProcessFile(sourceFile, tmpDir string) (string, error) {
	debugf("开始处理文件: %s", sourceFile)

	// 读取源文件
	content, err := ioutil.ReadFile(sourceFile)
	if err != nil {
		return "", fmt.Errorf("读取源文件失败: %v", err)
	}

	// 解析源文件
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, sourceFile, content, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("解析源文件失败: %v", err)
	}

	// 检查是否需要处理注解
	modified := false
	ast.Inspect(file, func(n ast.Node) bool {
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			if funcDecl.Doc != nil {
				annotations := parseAnnotations(funcDecl.Doc)
				debugf("函数 %s 的注解: %v", funcDecl.Name.Name, annotations)
				if len(annotations) > 0 {
					if err := wrapFunction(file, funcDecl, annotations); err != nil {
						debugf("包装函数失败 %s: %v", funcDecl.Name.Name, err)
						return false
					}
					modified = true
				}
			}
		}
		return true
	})

	if !modified {
		debugf("文件无需处理")
		return sourceFile, nil
	}

	// 添加必要的导入
	addRequiredImports(file)

	// ���成新文件
	newFile := filepath.Join(tmpDir, filepath.Base(sourceFile))
	debugf("生成新文件: %s", newFile)

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(newFile), 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %v", err)
	}

	// 写入新文件
	f, err := os.Create(newFile)
	if err != nil {
		return "", fmt.Errorf("创建新文件失败: %v", err)
	}
	defer f.Close()

	if err := printer.Fprint(f, fset, file); err != nil {
		return "", fmt.Errorf("写入新文件失败: %v", err)
	}

	debugf("文件处理完成: %s", newFile)
	return newFile, nil
}

// 解析注解
func parseAnnotations(comments *ast.CommentGroup) []string {
	var annotations []string
	if comments == nil {
		return annotations
	}

	for _, c := range comments.List {
		text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if strings.HasPrefix(text, "@") {
			// 提取注解名称和参数
			text = strings.TrimPrefix(text, "@")
			// 分离注解和注释
			parts := strings.SplitN(text, " ", 2)
			annotation := parts[0]

			// 处理带参数的注解
			if strings.Contains(annotation, "(") {
				annotations = append(annotations, annotation)
			} else {
				// 不带参数的注解
				annotations = append(annotations, strings.TrimSpace(annotation))
			}
		}
	}
	return annotations
}

// 添加必要的导入
func addRequiredImports(file *ast.File) {
	imports := map[string]string{
		"github.com/sjqzhang/gdi": "gdi",
		"time":                    "time",
	}

	for path, name := range imports {
		addImport(file, path, name)
	}
}

// 添加单个导入
func addImport(file *ast.File, path, name string) {
	// 检查是否已经导入
	for _, imp := range file.Imports {
		if imp.Path.Value == `"`+path+`"` {
			return
		}
	}

	// 添加的导入
	newImport := &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: `"` + path + `"`,
		},
	}

	// 如果需要别名
	if name != "" && name != filepath.Base(path) {
		newImport.Name = ast.NewIdent(name)
	}

	// 添加到文件的导入声明中
	if len(file.Imports) == 0 {
		file.Imports = []*ast.ImportSpec{newImport}
		file.Decls = append([]ast.Decl{
			&ast.GenDecl{
				Tok:   token.IMPORT,
				Specs: []ast.Spec{newImport},
			},
		}, file.Decls...)
	} else {
		for _, decl := range file.Decls {
			if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
				genDecl.Specs = append(genDecl.Specs, newImport)
				break
			}
		}
	}
}