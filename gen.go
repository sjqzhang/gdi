package gdi

import (
	"bytes"
	"embed"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var sources map[string][]string = make(map[string][]string)
var packSources map[string][]string = make(map[string][]string)
var restMap map[string]restInfo = make(map[string]restInfo)

var pkgName string

func init() {
	sources = make(map[string][]string)
	packSources = make(map[string][]string)
	restMap = make(map[string]restInfo)
	sources = getGoSources()
}

func listFiles(fsys *embed.FS, fpath string, fsMap map[string][]string) error {
	files, err := fs.ReadDir(fsys, fpath)
	if err != nil {
		globalGDI.error(fmt.Sprintf("read dir error: %s", err.Error()))
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			//fmt.Printf("Directory: %s\n", file.Name())
			if fpath != "." {
				err = listFiles(fsys, fpath+"/"+file.Name(), fsMap)
			} else {
				err = listFiles(fsys, file.Name(), fsMap)
			}
			if err != nil {
				return err
			}
		} else {
			if !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") {
				continue
			}
			dirname := fpath
			if fpath != "." {
				dirname = fpath + "/" + file.Name()
			} else {
				dirname = file.Name()
			}

			dir := path.Dir(dirname)
			if _, ok := fsMap[dir]; !ok {
				fsMap[dir] = make([]string, 0)
			}
			content, err := fsys.ReadFile(dirname)
			if err != nil {
				continue
			}
			fsMap[dir] = append(fsMap[dir], string(content))

		}
	}
	//fmt.Println(fsMap)
	return nil
}

func runCmd(cmds ...string) string {
	if len(cmds) < 1 {
		return ""
	}
	var out bytes.Buffer
	cmd := exec.Command(cmds[0], cmds[1:]...)
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Run()
	return out.String()
}

func getAllPackages() []string {

	packages := runCmd("go", "list", "./...")

	return strings.Split(packages, "\n")
}

func getDir() string {

	return runCmd("go", "list", "-f", "{{.Dir}}")

}

func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	commonPrefix := ""
	for i := 0; i < len(strs[0]); i++ {
		char := strs[0][i]

		for j := 1; j < len(strs); j++ {
			if i >= len(strs[j]) || strs[j][i] != char {
				return commonPrefix
			}
		}
		commonPrefix += string(char)
	}
	return commonPrefix
}

func GetAppModuleName() string { //TODO：通过比较包路径，获取包名，不一定准确
	return globalGDI.GetAppModuleName()
}

func (gdi *GDIPool) GetAppModuleName() string { //TODO：通过比较包路径，获取包名，不一定准确
	if len(globalGDI.placeHolders) == 0 {
		gdi.error("you must register at least three placeholder,you can call gdi.GenGDIRegisterFile(true) to register")
		return ""
	}
	if !strings.Contains(pkgName, " ") && pkgName != "" {
		return pkgName
	}
	var pkgPaths []string
	for _, v := range globalGDI.placeHolders {
		if reflect.TypeOf(v).Elem().PkgPath() != "" {
			pkgPaths = append(pkgPaths, reflect.TypeOf(v).Elem().PkgPath())
		}
	}
	pkgName = strings.TrimSuffix(longestCommonPrefix(pkgPaths), "/")
	return pkgName
}

func getGoSources() map[string][]string {
	packagePath := runCmd("go", "list", "-f", "{{.Module}}", "./...")
	packagePath = strings.TrimSpace(strings.Split(packagePath, "\n")[0])
	pkgName = packagePath
	packages := getAllPackages()

	goFiles := make(map[string][]string)
	baseDir := strings.TrimSpace(getDir())
	if !strings.HasPrefix(baseDir, "/") {
		baseDir, _ = os.Getwd()
	}
	for _, p := range packages {
		dir := strings.TrimPrefix(p, packagePath)
		if dir == "" {
			continue
		}
		dirPrefix := baseDir + "/" + dir
		fs, err := ioutil.ReadDir(dirPrefix)

		if err != nil {
			fmt.Println(err)
		}
		//var gos []string
		var orginGors []string
		for _, f := range fs {
			if strings.HasSuffix(f.Name(), ".go") && !strings.HasSuffix(f.Name(), "_test.go") {
				bs, err := ioutil.ReadFile(dirPrefix + "/" + f.Name())
				if err != nil {
					fmt.Println(err)
					continue

				}
				source := string(bs)
				orginGors = append(orginGors, source)

			}

		}
		packSources[strings.Trim(dir, "/")] = orginGors
		goFiles[p] = orginGors

	}

	return goFiles

}

func getImportSource() map[string][]string {
	goFiles := make(map[string][]string)
	reg := regexp.MustCompile(`package\s+main\s*$`)
	comment := regexp.MustCompile(`/\*{1,2}[\s\S]*?\*/|//[\s\S]*?\n`) //remove comment
	regBrackets := regexp.MustCompile("`[^`]+?`|{[^{|}]*}")           //remove {}
	for p, files := range sources {
		var gos []string
		for _, source := range files {
			source = comment.ReplaceAllString(source, "")
			source = strings.TrimSpace(source)
			lines := strings.Split(source, "\n")
			if reg.MatchString(lines[0]) {
				continue //ignore main package
				//p = "."
			}

			for i := 0; i < 100; i++ {
				old := len(source)
				source = regBrackets.ReplaceAllString(source, "")
				if len(source) == old {
					break
				}
			}
			gos = append(gos, source)
		}
		goFiles[p] = gos

	}

	return goFiles

}

//如果是go1.16以上，使用go embed
func checkGoVersion(version string) bool {
	//判断go版本是否大于指定版本,大于指定版本返回true
	curVersion := runtime.Version()
	if len(curVersion) > len(version) {
		curVersion = curVersion[0 : len(version)-1]
	}
	if compareVersion(curVersion, version) >= 0 {
		return true
	}
	return false
}

// compareVersion 比较两个版本号的大小
// 返回值：
//   -1 表示 version1 < version2
//    0 表示 version1 = version2
//    1 表示 version1 > version2
func compareVersion(version1, version2 string) int {
	parts1 := strings.Split(version1, ".")
	parts2 := strings.Split(version2, ".")

	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		num1, _ := strconv.Atoi(parts1[i])
		num2, _ := strconv.Atoi(parts2[i])

		if num1 < num2 {
			return -1
		} else if num1 > num2 {
			return 1
		}
	}

	// 版本号长度不一致时，较长部分为大
	if len(parts1) < len(parts2) {
		return -1
	} else if len(parts1) > len(parts2) {
		return 1
	}

	return 0
}

func genDependency() string {
	packages := getImportSource()
	reg := regexp.MustCompile(`type\s+([A-Z]\w+)\s+struct`)

	var aliasPack []string
	var allPacks []string
	for p, _ := range packages {
		allPacks = append(allPacks, p)

	}
	sort.Strings(allPacks)
	var regFuncs []string
	index := 0
	for _, p := range allPacks {
		sources := packages[p]
		if len(sources) == 0 {
			continue
		}
		bflag := false
		index++
		for _, source := range sources {
			matches := reg.FindAllStringSubmatch(source, -1)
			for _, m := range matches {
				if len(m) == 2 {
					bflag = true
					if p == "." {
						regFuncs = append(regFuncs, fmt.Sprintf("gdi.PlaceHolder((*%v)(nil))", m[1]))
					} else {
						regFuncs = append(regFuncs, fmt.Sprintf("gdi.PlaceHolder((*p%v.%v)(nil))", index, m[1]))
					}
				}
			}
		}
		if bflag && p != "." {
			aliasPack = append(aliasPack, fmt.Sprintf(`p%v "%v"`, index, p))
		}
	}

	tpl := `package main
/*
重要说明：本文件由gdi自动生成，请勿手动修改，当被注册的结构体发生变化编译出错时，请删除本文件或清除所有文件内容，只需要保留本包声明即可。(注意：
在没有生成代码的情况下，第一次运行程序会报错，找不到包声明，需要进行二次编译)
Important note: This file is automatically generated by gdi, do not modify it manually. 
If the registered structure changes and the compilation fails, please delete this file or clear all the contents of the file.
Only the package declaration is required.(Note: 
In the case of no generated code, the first run of the program will report an error, can not find the package declaration, 
need to do a secondary compilation)
*/
import (
	%v
	"github.com/sjqzhang/gdi"
)

func init() {
     _=gdi.GDIPool{}
	%v
}

`

	bflag := false

	if checkGoVersion("go1.16") {
		tpl = `package main
/*
重要说明：本文件由gdi自动生成，请勿手动修改，当被注册的结构体发生变化编译出错时，请删除本文件或清除所有文件内容，只需要保留本包声明即可。
Important note: This file is automatically generated by gdi, do not modify it manually. 
If the registered structure changes and the compilation fails, please delete this file or clear all the contents of the file.
Only the package declaration is required.(Note: 
In the case of no generated code, the first run of the program will report an error, can not find the package declaration, 
need to do a secondary compilation)
*/
import (
	%v
	"github.com/sjqzhang/gdi"
	"embed"
)

//go:embed %v
var gdiEmbedFiles embed.FS

func init() {
	gdi.SetEmbedFs(&gdiEmbedFiles)
     _=gdi.GDIPool{}
	%v
}

`
		bflag = true

	}

	importPackages := strings.Join(aliasPack, "\n")
	registerFun := strings.Join(regFuncs, "\n")

	var ps []string
	for p, _ := range packSources {
		ps = append(ps, p)
	}
	pss := strings.Join(ps, " ")
	if bflag {
		return fmt.Sprintf(tpl, importPackages, pss, registerFun)
	}
	return fmt.Sprintf(tpl, importPackages, registerFun)

}

func GenGDIRegisterFile(override bool) {
	globalGDI.GenGDIRegisterFile(override)
}

func getCurrentAbPathByCaller(skip int) string {
	var abPath string
	_, filename, _, ok := runtime.Caller(skip)
	if ok {
		abPath = path.Dir(filename)
	}
	return abPath
}

func (gdi *GDIPool) GenGDIRegisterFile(override bool) {
	fn := getCurrentAbPathByCaller(3) + "/gdi_gen.go"
	if _, err := os.Stat(fn); err != nil {
		ioutil.WriteFile(fn, []byte(genDependency()), 0755)
	} else {
		if override {
			content := genDependency()
			if strings.Contains(content, "gdi.PlaceHolder") { //如果不存在自动导入，没有必要覆盖
				ioutil.WriteFile(fn, []byte(content), 0755)
			}
		}
	}
	runCmd("gofmt", "-w", fn)

}

func GetRouterInfoByPatten(packagePatten string) (map[string]RouterInfo, error) {
	return globalGDI.GetRouterInfoByPatten(packagePatten)
}

func GetRestInfoByPatten(packageName string) (map[string]restInfo, error) {
	return globalGDI.GetRestInfoByPatten(packageName)
}

type middleware struct {
	Name   string
	Params sync.Map
}

type RouterInfo struct {
	Uri         string       `json:"uri"`
	PkgName     string       `json:"pkg_name"`
	PkgPath     string       `json:"pkg_path"`
	Method      string       `json:"method"`
	Controller  string       `json:"controller"`
	Handler     string       `json:"handler"`
	Middlewares []middleware `json:"middlewares"`
	Description string       `json:"description"`
	//RestInfo    *restInfo
}

type restInfo struct {
	Uri         string       `json:"uri"`
	PkgName     string       `json:"pkg_name"`
	PkgPath     string       `json:"pkg_path"`
	Controller  string       `json:"controller"`
	Middlewares []middleware `json:"middlewares"`
	Description string       `json:"description"`
}

func parseMiddleware(sourceCode string) map[string][]string {
	middlewares := make(map[string][]string)
	regMiddle := regexp.MustCompile(`//@middleware\s+([^\n]+)\n+\s*type\s+([\w]+)\s+struct`)
	regSplit := regexp.MustCompile(`[\s,]+`)
	matches := regMiddle.FindAllStringSubmatch(sourceCode, -1)
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}
		middlewares[strings.TrimSpace(match[2])] = regSplit.Split(match[1], -1)
	}
	return middlewares
}
func parseMiddlewareAnnotations(annotations string) []middleware {

	var middlewares []middleware

	// 分隔多个中间件
	middlewareList := strings.Split(annotations, ";")

	// 正则表达式用于匹配参数
	paramRegexMatch := regexp.MustCompile(`(\([^\)]+\))`)

	paramRegex := regexp.MustCompile(`(\w+)\s*=\s*("[^"]+?")|(\w+)\s*=\s*('[^']+?')|(\w+)\s*=\s*([^,]+)`)

	for _, middle := range middlewareList {
		params := paramRegexMatch.FindString(middle)
		if params != "" {
			middle = middle[:len(middle)-len(params)]
			params = params[1 : len(params)-1]
		}
		// 解析参数  ttl=5,prefix="news",key="{id},{title}" 成为map,注意value是字符串中带逗号的情况
		paramMap := sync.Map{}
		if params != "" {
			paramList := paramRegex.FindAllString(params, -1)
			for _, param := range paramList {
				kv := strings.Split(param, "=")
				if len(kv) == 2 {
					paramMap.Store(kv[0], strings.TrimSpace(strings.Trim(kv[1], "\"'")))
					//paramMap[kv[0]] = strings.TrimSpace(strings.Trim(kv[1], "\"'"))
				}
			}
		}
		middle = strings.TrimSpace(middle)
		if middle == "" {
			continue
		}
		middlewares = append(middlewares, middleware{
			Name:   middle,
			Params: paramMap,
		})
	}

	return middlewares
}

var regexMiddlewarePrefix = regexp.MustCompile(`(?i)^\s*@middleware`)
var regexDescriptionPrefix = regexp.MustCompile(`(?i)^\s*@description`)
var regexRouterPrefix = regexp.MustCompile(`(?i)^\s*@router`)

func parseRouterInfo(sourceCode string, pkgPath string) ([]RouterInfo, error) {
	//trim empty line
	lines := strings.Split(sourceCode, "\n")
	var newLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			newLines = append(newLines, line)
		}
	}
	sourceCode = strings.Join(newLines, "\n")
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", sourceCode, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var routerInfos []RouterInfo
	var currentRouterInfo RouterInfo
	var rest restInfo

	for _, decl := range f.Decls {
		switch d := decl.(type) {

		case *ast.GenDecl:
			rest = restInfo{}
			if d.Doc != nil {
				for _, comment := range d.Doc.List {
					if strings.TrimSpace(comment.Text) == "" {
						continue
					}
					text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
					if regexRouterPrefix.MatchString(text) {
						routerInfo := parseRouterComment(text)
						if routerInfo != nil {
							rest.Uri = routerInfo.Uri
						}
					} else if regexMiddlewarePrefix.MatchString(text) {
						middlewares := parseMiddlewareAnnotations(regexMiddlewarePrefix.ReplaceAllString(text, ""))
						rest.Middlewares = middlewares
					} else if regexDescriptionPrefix.MatchString(text) {
						rest.Description = strings.TrimSpace(regexDescriptionPrefix.ReplaceAllString(text, ""))
					}
				}
			}

			for _, spec := range d.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					if structType, ok := ts.Type.(*ast.StructType); ok {
						rest.Controller = ts.Name.Name
						rest.PkgPath = pkgPath
						rest.PkgName = pkgName
						restMap[fmt.Sprintf("%v.%v", pkgPath, rest.Controller)] = rest
						_ = structType
					}
				}
			}

		case *ast.FuncDecl:
			currentRouterInfo = RouterInfo{}
			if d.Doc != nil {
				for _, comment := range d.Doc.List {
					if strings.TrimSpace(comment.Text) == "" {
						continue
					}
					text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
					if regexRouterPrefix.MatchString(text) {
						routerInfo := parseRouterComment(text)
						if routerInfo != nil {
							tmp := *routerInfo
							if tmp.Uri != "" {
								currentRouterInfo.Uri = tmp.Uri
								currentRouterInfo.Method = tmp.Method
							}
						}
					} else if regexMiddlewarePrefix.MatchString(text) {
						middlewares := parseMiddlewareAnnotations(regexMiddlewarePrefix.ReplaceAllString(text, ""))
						currentRouterInfo.Middlewares = middlewares
					} else if regexDescriptionPrefix.MatchString(text) {
						currentRouterInfo.Description = strings.TrimSpace(regexDescriptionPrefix.ReplaceAllString(text, ""))
					}
				}
			}
			if currentRouterInfo.Handler == "" {
				currentRouterInfo.Handler = d.Name.String()
			}
			if currentRouterInfo.Controller == "" && d != nil {
				currentRouterInfo.Controller = fmt.Sprintf("%v.%v", pkgPath, extractControllerName(d, sourceCode))
			}

			if currentRouterInfo.Handler != "" && currentRouterInfo.Controller != "" {
				if currentRouterInfo.Method == "" {
					currentRouterInfo.Method = "ANY"
				}
				currentRouterInfo.PkgPath = pkgPath
				currentRouterInfo.PkgName = pkgName
				routerInfos = append(routerInfos, currentRouterInfo)
			}

			//if currentRouterInfo.Uri != "" && currentRouterInfo.Method != "" && currentRouterInfo.Controller != "" {
			//	//currentRouterInfo.RestInfo = &restInfo
			//	if rest.Uri != "" {
			//		currentRouterInfo.Uri = rest.Uri + currentRouterInfo.Uri
			//	}
			//	if currentRouterInfo.Middlewares == nil || len(currentRouterInfo.Middlewares) == 0 && currentRouterInfo.Controller == rest.Controller {
			//		currentRouterInfo.Middlewares = rest.Middlewares
			//	}
			//
			//}

		}
	}

	return routerInfos, nil
}

func parseRouterComment(comment string) *RouterInfo {
	parts := strings.Fields(comment)
	if len(parts) >= 3 {
		return &RouterInfo{
			Uri:    parts[1],
			Method: strings.ToUpper(strings.Trim(parts[2], "[ ]")),
		}
	} else if len(parts) == 2 {
		return &RouterInfo{
			Uri:    parts[1],
			Method: "ANY",
		}
	}
	return nil
}

func parseMiddlewareComment(comment string) []string {
	reg := regexp.MustCompile(`\s*@middleware\s+([^\n]+)`)
	parts := reg.FindAllStringSubmatch(comment, -1)
	if len(parts) > 0 && len(parts[0]) > 0 {
		return strings.Split(strings.Replace(parts[0][1], " ", "", -1), ",")
	}
	return nil
}

func extractControllerName(decl *ast.FuncDecl, sourceCode string) string {
	recv := decl.Recv
	if recv == nil {
		return ""
	}
	if len(recv.List) > 0 {
		field := recv.List[0]
		if len(field.Names) > 0 {
			n := sourceCode[recv.Pos()-1 : recv.End()-1]
			ns := strings.Split(n, " ")
			if len(ns) > 1 {
				return strings.Trim(ns[1], "*) ")
			}
			recvName := field.Names[0].String()
			if strings.Contains(recvName, "*") {
				recvName = strings.TrimPrefix(recvName, "*")
			}
			return recvName
		}
	}
	return ""
}

//func parseRouterInfo2(sourceCode string) ([]RouterInfo, error) {
//	var routerInfos []RouterInfo
//	regex := regexp.MustCompile(`func\s*\(([^)]+)\)\s+([\w]+)`)
//	matches := regex.FindAllStringSubmatch(sourceCode, -1)
//	spaceReg := regexp.MustCompile(`\s+\*?`)
//	//Http Method Match Regex
//	methodReg := regexp.MustCompile(`(Get|Post|Put|Delete|Head|Options|Patch|Any)`)
//	for _, match := range matches {
//		if len(match) != 3 {
//			continue
//		}
//		ctrlName := ""
//		controller := spaceReg.Split(match[1], -1)
//		if len(controller) > 1 {
//			ctrlName = controller[1]
//		}
//		if !strings.HasSuffix(ctrlName, "Controller") {
//			continue
//		}
//		uri := "/api/" + strings.ToLower(ctrlName[:len(ctrlName)-10]) + "/" + strings.TrimSpace(match[2])
//		//从方法名中获取HTTP方法,方法名格式为Get,Post,Put,Delete
//		methodMatch := methodReg.FindStringSubmatch(match[2])
//		method := "GET"
//		if len(methodMatch) > 1 {
//			method = strings.ToUpper(methodMatch[1])
//		}
//		routerInfo := RouterInfo{
//			Uri:        uri,
//			Method:     method,
//			Controller: strings.TrimSpace(ctrlName),
//			Handler:    strings.TrimSpace(match[2]),
//		}
//		routerInfos = append(routerInfos, routerInfo)
//	}
//
//	// 定义正则表达式，匹配格式为 // @router /uri [method]\nfunc (this *Controller) HandlerName()
//	regex = regexp.MustCompile(`//\s*@router\s+(\/\S+)\s+\[(\S+)\]\s*\nfunc\s*\(([^\)]+)\)\s*([\w]+)`)
//	matches = regex.FindAllStringSubmatch(sourceCode, -1)
//
//	for _, match := range matches {
//		if len(match) != 5 {
//			continue
//		}
//		ctrlName := ""
//		controller := spaceReg.Split(match[3], -1)
//		if len(controller) > 1 {
//			ctrlName = controller[1]
//		}
//		routerInfo := RouterInfo{
//			Uri:        strings.TrimSpace(match[1]),
//			Method:     strings.ToUpper(strings.TrimSpace(match[2])),
//			Controller: strings.TrimSpace(ctrlName),
//			Handler:    strings.TrimSpace(match[4]),
//		}
//		routerInfos = append(routerInfos, routerInfo)
//	}
//
//	middleMap := parseMiddleware(sourceCode)
//	for i, info := range routerInfos {
//		if v, ok := middleMap[info.Controller]; ok {
//			routerInfos[i].Middlewares = v
//		}
//	}
//	return routerInfos, nil
//}

func (gdi *GDIPool) genRouter(packageName string) ([]RouterInfo, error) {
	var routerInfos []RouterInfo
	if gdi.fs == nil {
		return nil, nil
	}
	files, err := gdi.fs.ReadDir(packageName)
	if err != nil {
		gdi.error(err.Error())
		return nil, err
	}

	for _, file := range files {
		byteContents, err := gdi.fs.ReadFile(fmt.Sprintf("%v/%v", packageName, file.Name()))
		if err != nil {
			continue
		}
		//fmt.Println(string(byteContents))
		infos, err := parseRouterInfo(string(byteContents), packageName)
		if err != nil {
			return infos, err
		}
		routerInfos = append(routerInfos, infos...)
	}
	// build router info
	for k, rest := range restMap {
		for i, route := range routerInfos {
			if route.Controller == k {
				if route.Uri == "" {
					route.Uri = fmt.Sprintf("/%v", route.Handler)
				}
				routerInfos[i].Uri = rest.Uri + route.Uri
				if len(route.Middlewares) == 0 {
					routerInfos[i].Middlewares = rest.Middlewares
				}
			}
		}
	}
	return routerInfos, nil
}

func (gdi *GDIPool) GetRouterInfoByPatten(packagePatten string) (map[string]RouterInfo, error) {
	var routerInfoMap = make(map[string]RouterInfo)
	packageNames := make(map[string]string)
	regPatten := regexp.MustCompile(packagePatten)
	for k, _ := range packSources {
		if regPatten.MatchString(k) {
			packageNames[k] = k
		}
	}
	for packageName, _ := range packageNames {

		routerInfos, err := gdi.genRouter(packageName)
		if err != nil {
			gdi.log(err.Error())
			return nil, err
		}
		for _, routerInfo := range routerInfos {
			routerInfoMap[routerInfo.Controller+"."+routerInfo.Handler] = routerInfo
		}

	}
	return routerInfoMap, nil

}

func (gdi *GDIPool) getRouterInfo(packageName string) (map[string]RouterInfo, error) {

	routerInfoMap := make(map[string]RouterInfo)

	routerInfos, err := gdi.genRouter(packageName)
	if err != nil {
		gdi.log(err.Error())
		return nil, err
	}
	for _, routerInfo := range routerInfos {
		routerInfoMap[routerInfo.Controller+"."+routerInfo.Handler] = routerInfo
	}

	return routerInfoMap, nil
}

func SetEmbedFs(fs *embed.FS) {
	globalGDI.SetEmbedFs(fs)
}

func (gdi *GDIPool) SetEmbedFs(fs *embed.FS) {
	if fs == nil {
		return
	}
	gdi.fs = fs
	if len(packSources) == 0 {
		packSources = make(map[string][]string)
		listFiles(fs, ".", packSources)
	}
}

func (gdi *GDIPool) getFileConent(filePath string) ([]byte, error) {
	if gdi.fs == nil {
		return nil, fmt.Errorf("check go version>go1.16 and call gdi.GenGDIRegisterFile(true) first!")
	}
	return gdi.fs.ReadFile(filePath)
}

func (gdi *GDIPool) GetFileConent(filePath string) ([]byte, error) {
	return gdi.getFileConent(filePath)
}

func GetFileConent(filePath string) ([]byte, error) {
	return globalGDI.getFileConent(filePath)
}

func (gdi *GDIPool) getAstTree(filePath string) (*ast.File, error) {

	fset := token.NewFileSet()
	byteCode, err := gdi.getFileConent(filePath)
	if err != nil {
		return nil, err
	}
	return parser.ParseFile(fset, "", byteCode, parser.ParseComments)

}

func (gdi *GDIPool) GetAstTree(filePath string) (*ast.File, error) {
	return gdi.getAstTree(filePath)
}

func GetAstTree(filePath string) (*ast.File, error) {
	return globalGDI.getAstTree(filePath)
}
