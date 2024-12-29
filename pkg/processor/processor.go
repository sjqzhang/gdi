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

	// 查找项目根目录（包含 go.mod 的目录）
	projectRoot := findProjectRoot(sourceFile)
	if projectRoot == "" {
		return "", fmt.Errorf("找不到项目根目录（go.mod 所在目录）")
	}

	// 计算源文件相对于项目根目录的路径
	relPath, err := filepath.Rel(projectRoot, sourceFile)
	if err != nil {
		return "", fmt.Errorf("计算相对路径失败: %v", err)
	}

	// 修正目录名拼写错误
	relPath = strings.Replace(relPath, "exmaple", "example", -1)

	// 在临时目录中创建相同的目录结构
	newFile := filepath.Join(tmpDir, relPath)
	if err := os.MkdirAll(filepath.Dir(newFile), 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %v", err)
	}

	// 复制项目文件到临时目录
	if err := copyProjectFiles(projectRoot, tmpDir); err != nil {
		debugf("复制项目文件失败: %v", err)
	}

	// 复制依赖包到临时目录
	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		srcPath := filepath.Join(gopath, "src")
		if err := copyDir(srcPath, filepath.Join(tmpDir, "src")); err != nil {
			debugf("复制依赖包失败: %v", err)
		}
	}

	debugf("生成新文件: %s", newFile)

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

// findProjectRoot 查找包含 go.mod 的项目根目录
func findProjectRoot(start string) string {
	dir := filepath.Dir(start)
	for dir != "/" && dir != "." {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

// copyProjectFiles 复制项目必要文件到临时目录
func copyProjectFiles(src, dst string) error {
	// 复制 go.mod
	if err := copyFile(filepath.Join(src, "go.mod"), filepath.Join(dst, "go.mod")); err != nil {
		return fmt.Errorf("复制 go.mod 失败: %v", err)
	}

	// 复制 go.sum（如果存在）
	if _, err := os.Stat(filepath.Join(src, "go.sum")); err == nil {
		if err := copyFile(filepath.Join(src, "go.sum"), filepath.Join(dst, "go.sum")); err != nil {
			return fmt.Errorf("复制 go.sum 失败: %v", err)
		}
	}

	return nil
}

// copyFile 复制单个文件
func copyFile(src, dst string) error {
	input, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dst, input, 0644)
}

// 解析注解
func parseAnnotations(comments *ast.CommentGroup) []Annotation {
	var annotations []Annotation
	if comments == nil {
		return annotations
	}

	for _, c := range comments.List {
		text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if strings.HasPrefix(text, "go:gdi") {
			// 提取注解名称和参数
			text = strings.TrimPrefix(text, "go:gdi")
			text = strings.TrimSpace(text)

			// 分离注解和注释
			parts := strings.SplitN(text, " ", 2)
			annotationText := parts[0]

			// 解析注解和参数
			var annotation Annotation
			if strings.Contains(annotationText, "(") {
				// 带参数的注解
				name := strings.Split(annotationText, "(")[0]
				paramsStr := strings.TrimSuffix(strings.Split(annotationText, "(")[1], ")")
				params := make(map[string]string)

				// 解析参数
				if paramsStr != "" {
					paramPairs := strings.Split(paramsStr, ",")
					for _, pair := range paramPairs {
						pair = strings.TrimSpace(pair)
						kv := strings.Split(pair, "=")
						if len(kv) == 2 {
							key := strings.Trim(strings.TrimSpace(kv[0]), `"`)
							value := strings.Trim(strings.TrimSpace(kv[1]), `"`)
							params[key] = value
						}
					}
				}

				annotation = Annotation{
					Name:   name,
					Params: params,
				}
			} else {
				// 不带参数的注解
				annotation = Annotation{
					Name:   strings.TrimSpace(annotationText),
					Params: make(map[string]string),
				}
			}

			annotations = append(annotations, annotation)
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

// copyDir 递归复制目录
func copyDir(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}
