package gdi

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

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

	//fmt.Println(packages)

	return strings.Split(packages, "\n")
}

func getDir() string {

	return runCmd("go", "list", "-f", "{{.Dir}}")

}

func getGoSources() map[string][]string {
	packagePath := ""
	packages := getAllPackages()
	if len(packages) > 0 {
		packagePath = strings.Split(packages[0], "/")[0]
	}
	reg := regexp.MustCompile(`package\s+main\s*$`)
	comment := regexp.MustCompile(`/\*{1,2}[\s\S]*?\*/|//[\s\S]*?\n`)
	goFiles := make(map[string][]string)
	baseDir := strings.TrimSpace(getDir())
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
		var gos []string
		for _, f := range fs {
			if strings.HasSuffix(f.Name(), ".go") && !strings.HasSuffix(f.Name(), "_test.go") {
				bs, err := ioutil.ReadFile(dirPrefix + "/" + f.Name())
				if err != nil {
					fmt.Println(err)
					continue

				}
				source := string(bs)
				lines := strings.Split(source, "\n")
				if reg.MatchString(lines[0]) {
					continue
				}
				source = comment.ReplaceAllString(source, "")
				gos = append(gos, source)
			}

		}
		goFiles[p] = gos

	}

	return goFiles

}

func genDependency() string {
	packages := getGoSources()
	reg := regexp.MustCompile(`type\s+([A-Z]\w+)\s+struct`)

	var aliasPack []string
	var allPacks []string
	for p,_:=range packages {
		allPacks=append(allPacks,p)

	}
	sort.Strings(allPacks)
	var regFuncs []string
	index := 0
	for _, p := range allPacks {
		sources:= packages[p]
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
					regFuncs = append(regFuncs, fmt.Sprintf("gdi.PlaceHolder(p%v.%v{})", index, m[1]))
				}
			}
		}
		if bflag {
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


	importPackages := strings.Join(aliasPack, "\n")
	registerFun := strings.Join(regFuncs, "\n")

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
	source := genDependency()
	if _, err := os.Stat(fn); err != nil {
		ioutil.WriteFile(fn, []byte(source), 0755)
	} else {
		if override {
			ioutil.WriteFile(fn, []byte(source), 0755)
		}
	}

}
