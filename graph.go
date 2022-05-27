package gdi

import (
	"fmt"
	"strings"
)

var g *graph

func init() {
	g = &graph{
		nodes: make(map[string]*node),
	}
}

type node struct {
	name   string
	fields []*nodeItem
	edges  []*edge
	//fields map[string]string
	//edges  map[string]string
}

type edge struct {
	from string
	to   string
}

type nodeItem struct {
	fieldName string
	fieldType string
}

type graph struct {
	nodes map[string]*node
}

func (n *node) addFiled(f *nodeItem) {
	//if n.fields ==nil {
	//	n.fields =make(map[string]string)
	//}
	//n.fields[f.fieldType]=f.fieldName
	n.fields=append(n.fields,f)
}
func (n *node) addEdge(e *edge) {
	//if n.edges==nil {
	//	n.edges=make(map[string]string)
	//}
	//n.edges[e.from]=e.to
	n.edges=append(n.edges,e)
}


func (g *graph) add(n *node) {
	g.nodes[n.name] = n
}

func (gdi *GDIPool) Graph() string {
	var gs []string
	/*
		"node0" [
		label = "<f0> 0x10ba8| <f1>"
		shape = "record"
		];
	*/
	nodeTpl := `
   "%v" [
     label = "%v"
     shape = "record"
 ]
`
	for _, n := range g.nodes {
		var fields []string
		fields = append(fields, fmt.Sprintf("<f100> struct %v", n.name))
		for _, field := range n.fields {
			fields = append(fields, fmt.Sprintf(`%v %v`, field.fieldName,field.fieldType))
		}
		gs=append(gs,	fmt.Sprintf(nodeTpl,n.name,strings.Join(fields,"|")))
		var edges []string
		for _, e :=range  n.edges {
			edges = append(edges, fmt.Sprintf(`%v->"%v":f100;`, e.from, e.to))
		}
		gs=append(gs,strings.Join(edges,"\n"))


	}
	return strings.Join(gs,"\n")
}
