package gdi

import (
	"bytes"
	"embed"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

var sources map[string][]string = make(map[string][]string)
var packSources map[string][]string = make(map[string][]string)

func init() {
	sources = getGoSources()
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

func getGoSources() map[string][]string {
	packagePath := runCmd("go", "list", "-f", "{{.Module}}", "./...")
	packagePath = strings.TrimSpace(strings.Split(packagePath, "\n")[0])
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

import (
	%v
	"github.com/sjqzhang/gdi"
	"embed"
)

//go:embed %v
var gdiEmbededFiles embed.FS

func init() {
	gdi.SetEmbedFs(&gdiEmbededFiles)
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
			ioutil.WriteFile(fn, []byte(genDependency()), 0755)
		}
	}
	runCmd("gofmt", "-w", fn)

}

func GetRouterInfo(packageName string) (map[string]RouterInfo, error) {
	return globalGDI.GetRouterInfo(packageName)
}

type RouterInfo struct {
	Uri         string   `json:"uri"`
	Method      string   `json:"method"`
	Controller  string   `json:"controller"`
	Handler     string   `json:"handler"`
	Middlewares []string `json:"middlewares"`
	//RestInfo    *restInfo
}

type restInfo struct {
	Uri         string   `json:"uri"`
	Controller  string   `json:"controller"`
	Middlewares []string `json:"middlewares"`
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

func parseRouterInfo(sourceCode string) ([]RouterInfo, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", sourceCode, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var routerInfos []RouterInfo
	var currentRouterInfo RouterInfo
	var restInfo restInfo

	for _, decl := range f.Decls {
		switch d := decl.(type) {

		case *ast.GenDecl:
			if d.Doc != nil {
				for _, comment := range d.Doc.List {
					text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
					if strings.HasPrefix(text, "@router") {
						routerInfo := parseRouterComment(text)
						if routerInfo != nil {
							restInfo.Uri = routerInfo.Uri
						}
					} else if strings.HasPrefix(text, "@middleware") {
						middlewares := parseMiddlewareComment(text)
						restInfo.Middlewares = middlewares
					}
				}
			}

			for _, spec := range d.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					if structType, ok := ts.Type.(*ast.StructType); ok {
						restInfo.Controller = ts.Name.Name
						_ = structType
					}
				}
			}

		case *ast.FuncDecl:
			if d.Doc != nil {
				for _, comment := range d.Doc.List {
					text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
					if strings.HasPrefix(text, "@router") {
						routerInfo := parseRouterComment(text)
						if routerInfo != nil {
							currentRouterInfo = *routerInfo
						}
					} else if strings.HasPrefix(text, "@middleware") {
						middlewares := parseMiddlewareComment(text)
						currentRouterInfo.Middlewares = middlewares
					}
				}
			}
			if currentRouterInfo.Handler == "" {
				currentRouterInfo.Handler = d.Name.String()
			}
			if currentRouterInfo.Controller == "" && d != nil {
				currentRouterInfo.Controller = extractControllerName(d, sourceCode)
			}

			if currentRouterInfo.Uri != "" && currentRouterInfo.Method != "" && currentRouterInfo.Controller == restInfo.Controller {
				//currentRouterInfo.RestInfo = &restInfo
				if restInfo.Uri!="" {
					currentRouterInfo.Uri= restInfo.Uri+currentRouterInfo.Uri
				}
				if currentRouterInfo.Middlewares == nil || len(currentRouterInfo.Middlewares) == 0 {
					currentRouterInfo.Middlewares = restInfo.Middlewares
				}
				routerInfos = append(routerInfos, currentRouterInfo)
			}

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
	}
	return nil
}

func parseMiddlewareComment(comment string) []string {
	parts := strings.Split(strings.TrimSpace(comment), " ")
	if len(parts) > 1 {
		return parts[1:]
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
			n := sourceCode[recv.Pos():recv.End()]
			ns := strings.Split(n, " ")
			if len(ns) > 1 {
				return strings.Trim(ns[1], ") ")
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

func parseRouterInfo2(sourceCode string) ([]RouterInfo, error) {
	var routerInfos []RouterInfo
	regex := regexp.MustCompile(`func\s*\(([^)]+)\)\s+([\w]+)`)
	matches := regex.FindAllStringSubmatch(sourceCode, -1)
	spaceReg := regexp.MustCompile(`\s+\*?`)
	//Http Method Match Regex
	methodReg := regexp.MustCompile(`(Get|Post|Put|Delete|Head|Options|Patch|Any)`)
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}
		ctrlName := ""
		controller := spaceReg.Split(match[1], -1)
		if len(controller) > 1 {
			ctrlName = controller[1]
		}
		if !strings.HasSuffix(ctrlName, "Controller") {
			continue
		}
		uri := "/api/" + strings.ToLower(ctrlName[:len(ctrlName)-10]) + "/" + strings.TrimSpace(match[2])
		//从方法名中获取HTTP方法,方法名格式为Get,Post,Put,Delete
		methodMatch := methodReg.FindStringSubmatch(match[2])
		method := "GET"
		if len(methodMatch) > 1 {
			method = strings.ToUpper(methodMatch[1])
		}
		routerInfo := RouterInfo{
			Uri:        uri,
			Method:     method,
			Controller: strings.TrimSpace(ctrlName),
			Handler:    strings.TrimSpace(match[2]),
		}
		routerInfos = append(routerInfos, routerInfo)
	}

	// 定义正则表达式，匹配格式为 // @router /uri [method]\nfunc (this *Controller) HandlerName()
	regex = regexp.MustCompile(`//\s*@router\s+(\/\S+)\s+\[(\S+)\]\s*\nfunc\s*\(([^\)]+)\)\s*([\w]+)`)
	matches = regex.FindAllStringSubmatch(sourceCode, -1)

	for _, match := range matches {
		if len(match) != 5 {
			continue
		}
		ctrlName := ""
		controller := spaceReg.Split(match[3], -1)
		if len(controller) > 1 {
			ctrlName = controller[1]
		}
		routerInfo := RouterInfo{
			Uri:        strings.TrimSpace(match[1]),
			Method:     strings.ToUpper(strings.TrimSpace(match[2])),
			Controller: strings.TrimSpace(ctrlName),
			Handler:    strings.TrimSpace(match[4]),
		}
		routerInfos = append(routerInfos, routerInfo)
	}

	middleMap := parseMiddleware(sourceCode)
	for i, info := range routerInfos {
		if v, ok := middleMap[info.Controller]; ok {
			routerInfos[i].Middlewares = v
		}
	}
	return routerInfos, nil
}

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
			gdi.log(err.Error())
			continue
		}
		infos, err := parseRouterInfo(string(byteContents))
		if err != nil {
			return infos, err
		}
		routerInfos = append(routerInfos, infos...)
	}
	return routerInfos, nil
}

func (gdi *GDIPool) GetRouterInfo(packageName string) (map[string]RouterInfo, error) {
	routerInfos, err := gdi.genRouter(packageName)
	if err != nil {
		return nil, err
	}
	routerInfoMap := make(map[string]RouterInfo)
	for _, routerInfo := range routerInfos {
		routerInfoMap[routerInfo.Controller+"."+routerInfo.Handler] = routerInfo
	}
	return routerInfoMap, nil
}

func SetEmbedFs(fs *embed.FS) {
	globalGDI.SetEmbedFs(fs)
}

func (gdi *GDIPool) SetEmbedFs(fs *embed.FS) {
	gdi.fs = fs
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
