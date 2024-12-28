package processor

import (
	"fmt"
	"go/ast"
	"go/token"
)

// wrapFunction implementation
func wrapFunction(file *ast.File, funcDecl *ast.FuncDecl, annotations []string) error {
	fmt.Printf("Wrapping function %s with annotations: %v\n", funcDecl.Name.Name, annotations)
	// 保存原始函数体
	originalBody := funcDecl.Body

	// 创建新的函数体
	newBody := &ast.BlockStmt{
		List: []ast.Stmt{
			// 创建上下文
			createContextStmt(funcDecl),
			// 设置参数
			setArgsStmt(funcDecl),
			// 执行前置处理
			createBeforeHandlersStmt(annotations),
			// 执行原始函数
			createOriginalCallStmt(originalBody),
			// 设置返回值
			setReturnsStmt(funcDecl),
			// 设置结束时间
			setEndTimeStmt(),
			// 执行后置处理
			createAfterHandlersStmt(annotations),
			// 返回结果
			createReturnStmt(funcDecl),
		},
	}

	// 更新函数体
	funcDecl.Body = newBody
	return nil
}

// 创建上下文变量
func createContextStmt(funcDecl *ast.FuncDecl) ast.Stmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("ctx")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.UnaryExpr{
				Op: token.AND,
				X: &ast.CompositeLit{
					Type: &ast.SelectorExpr{
						X:   ast.NewIdent("gdi"),
						Sel: ast.NewIdent("Context"),
					},
					Elts: []ast.Expr{
						&ast.KeyValueExpr{
							Key:   ast.NewIdent("Method"),
							Value: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf(`"%s"`, funcDecl.Name.Name)},
						},
						&ast.KeyValueExpr{
							Key: ast.NewIdent("StartTime"),
							Value: &ast.CallExpr{
								Fun: &ast.SelectorExpr{
									X:   ast.NewIdent("time"),
									Sel: ast.NewIdent("Now"),
								},
							},
						},
						&ast.KeyValueExpr{
							Key: ast.NewIdent("Properties"),
							Value: &ast.CallExpr{
								Fun: ast.NewIdent("make"),
								Args: []ast.Expr{
									&ast.MapType{
										Key:   ast.NewIdent("string"),
										Value: ast.NewIdent("interface{}"),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// 设置参数
func setArgsStmt(funcDecl *ast.FuncDecl) ast.Stmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{
			&ast.SelectorExpr{
				X:   ast.NewIdent("ctx"),
				Sel: ast.NewIdent("Args"),
			},
		},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{
			&ast.CompositeLit{
				Type: &ast.ArrayType{
					Elt: ast.NewIdent("interface{}"),
				},
				Elts: createArgsList(funcDecl),
			},
		},
	}
}

// 创建参数列表
func createArgsList(funcDecl *ast.FuncDecl) []ast.Expr {
	var args []ast.Expr
	if funcDecl.Type.Params != nil {
		for _, field := range funcDecl.Type.Params.List {
			for _, name := range field.Names {
				args = append(args, ast.NewIdent(name.Name))
			}
		}
	}
	return args
}

// 创建前置处理语句
func createBeforeHandlersStmt(annotations []string) ast.Stmt {
	var stmts []ast.Stmt
	for _, ann := range annotations {
		stmts = append(stmts, createHandlerCallStmt("beforeHandlers", ann))
	}
	return &ast.BlockStmt{List: stmts}
}

// 创建后置处理语句
func createAfterHandlersStmt(annotations []string) ast.Stmt {
	var stmts []ast.Stmt
	for _, ann := range annotations {
		stmts = append(stmts, createHandlerCallStmt("afterHandlers", ann))
	}
	return &ast.BlockStmt{List: stmts}
}

// 创建处理器调用语句
func createHandlerCallStmt(handlerMap, annotation string) ast.Stmt {
	return &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{
				ast.NewIdent("handler"),
				ast.NewIdent("ok"),
			},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{
				&ast.IndexExpr{
					X:     ast.NewIdent(handlerMap),
					Index: &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf(`"%s"`, annotation)},
				},
			},
		},
		Cond: ast.NewIdent("ok"),
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.IfStmt{
					Init: &ast.AssignStmt{
						Lhs: []ast.Expr{ast.NewIdent("err")},
						Tok: token.DEFINE,
						Rhs: []ast.Expr{
							&ast.CallExpr{
								Fun:  ast.NewIdent("handler"),
								Args: []ast.Expr{ast.NewIdent("ctx")},
							},
						},
					},
					Cond: &ast.BinaryExpr{
						X:  ast.NewIdent("err"),
						Op: token.NEQ,
						Y:  ast.NewIdent("nil"),
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.ReturnStmt{},
						},
					},
				},
			},
		},
	}
}

// 设置结束时间
func setEndTimeStmt() ast.Stmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{
			&ast.SelectorExpr{
				X:   ast.NewIdent("ctx"),
				Sel: ast.NewIdent("EndTime"),
			},
		},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   ast.NewIdent("time"),
					Sel: ast.NewIdent("Now"),
				},
			},
		},
	}
}

// 创建返回语句
func createReturnStmt(funcDecl *ast.FuncDecl) ast.Stmt {
	if funcDecl.Type.Results == nil || len(funcDecl.Type.Results.List) == 0 {
		return &ast.ReturnStmt{}
	}

	return &ast.ReturnStmt{
		Results: []ast.Expr{
			&ast.SelectorExpr{
				X:   ast.NewIdent("result"),
				Sel: ast.NewIdent("result"),
			},
		},
	}
}

// 执行原始函数调用
func createOriginalCallStmt(originalBody *ast.BlockStmt) ast.Stmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("result")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.CallExpr{
			Fun:  ast.NewIdent("originalFunc"),
			Args: []ast.Expr{},
		}},
	}
}

// 设置返回值
func setReturnsStmt(funcDecl *ast.FuncDecl) ast.Stmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{
			&ast.SelectorExpr{
				X:   ast.NewIdent("ctx"),
				Sel: ast.NewIdent("Returns"),
			},
		},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{
			ast.NewIdent("result"),
		},
	}
}
