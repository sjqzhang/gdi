package processor

import (
	"fmt"
	"go/ast"
	"go/token"
)

// wrapFunction 实现多层装饰器的包装
func wrapFunction(file *ast.File, funcDecl *ast.FuncDecl, annotations []Annotation) error {
	debugf("包装函数 %s，注解: %v", funcDecl.Name.Name, annotations)

	// 保存原始函数体
	originalBody := funcDecl.Body

	// 从内到外构建装饰器层
	currentBody := createOriginalFuncBody(originalBody)

	// 逆序遍历注解，从内到外构建装饰器
	for i := len(annotations) - 1; i >= 0; i-- {
		ann := annotations[i]
		currentBody = createDecoratorLayer(ann, currentBody, i, funcDecl)
	}

	// 更新函数体
	funcDecl.Body = currentBody
	return nil
}

// createOriginalFuncBody 创建最内层的原始函数调用
func createOriginalFuncBody(originalBody *ast.BlockStmt) *ast.BlockStmt {
	return &ast.BlockStmt{
		List: originalBody.List,
	}
}

// createDecoratorLayer 创建单层装饰器
func createDecoratorLayer(ann Annotation, innerBody *ast.BlockStmt, layerIndex int, funcDecl *ast.FuncDecl) *ast.BlockStmt {
	ctxName := fmt.Sprintf("ctx_%d", layerIndex)

	// 构建装饰器层的语句列表
	stmts := []ast.Stmt{
		// 创建上下文
		createContextStmt(ctxName, funcDecl, ann),
		// 设置参数
		createSetArgsStmt(ctxName, funcDecl),
	}

	// 如果有参数，添加参数设置
	if len(ann.Params) > 0 {
		stmts = append(stmts, createSetParamsStmt(ctxName, ann.Params))
	}

	// 添加前置处理
	stmts = append(stmts, createBeforeHandlerStmt(ctxName, ann.Name))

	// 包装内层函数调用
	stmts = append(stmts, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("result")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.FuncLit{
				Type: &ast.FuncType{
					Results: funcDecl.Type.Results,
				},
				Body: innerBody,
			},
		},
	})

	// 执行函数并获取结果
	stmts = append(stmts, &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("returnValue")},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun: ast.NewIdent("result"),
			},
		},
	})

	// 设置结束时间
	stmts = append(stmts, createSetEndTimeStmt(ctxName))

	// 设置返回值
	stmts = append(stmts, &ast.AssignStmt{
		Lhs: []ast.Expr{
			&ast.SelectorExpr{
				X:   ast.NewIdent(ctxName),
				Sel: ast.NewIdent("Returns"),
			},
		},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{
			&ast.CompositeLit{
				Type: &ast.ArrayType{
					Elt: ast.NewIdent("interface{}"),
				},
				Elts: []ast.Expr{ast.NewIdent("returnValue")},
			},
		},
	})

	// 添加后置处理
	stmts = append(stmts, createAfterHandlerStmt(ctxName, ann.Name))

	// 返回结果
	stmts = append(stmts, &ast.ReturnStmt{
		Results: []ast.Expr{ast.NewIdent("returnValue")},
	})

	return &ast.BlockStmt{List: stmts}
}

// createContextStmt 创建上下文变量
func createContextStmt(ctxName string, funcDecl *ast.FuncDecl, ann Annotation) ast.Stmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(ctxName)},
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

// createSetArgsStmt 设置参数
func createSetArgsStmt(ctxName string, funcDecl *ast.FuncDecl) ast.Stmt {
	args := []ast.Expr{}
	if funcDecl.Type.Params != nil {
		for _, field := range funcDecl.Type.Params.List {
			for _, name := range field.Names {
				args = append(args, ast.NewIdent(name.Name))
			}
		}
	}

	return &ast.AssignStmt{
		Lhs: []ast.Expr{
			&ast.SelectorExpr{
				X:   ast.NewIdent(ctxName),
				Sel: ast.NewIdent("Args"),
			},
		},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{
			&ast.CompositeLit{
				Type: &ast.ArrayType{
					Elt: ast.NewIdent("interface{}"),
				},
				Elts: args,
			},
		},
	}
}

// createSetParamsStmt 设置注解参数
func createSetParamsStmt(ctxName string, params map[string]string) ast.Stmt {
	assignments := []ast.Stmt{}
	for key, value := range params {
		assignments = append(assignments, &ast.AssignStmt{
			Lhs: []ast.Expr{
				&ast.IndexExpr{
					X: &ast.SelectorExpr{
						X:   ast.NewIdent(ctxName),
						Sel: ast.NewIdent("Properties"),
					},
					Index: &ast.BasicLit{
						Kind:  token.STRING,
						Value: fmt.Sprintf(`"%s"`, key),
					},
				},
			},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{
				&ast.BasicLit{
					Kind:  token.STRING,
					Value: fmt.Sprintf(`"%s"`, value),
				},
			},
		})
	}
	return &ast.BlockStmt{List: assignments}
}

// createBeforeHandlerStmt 创建前置处理
func createBeforeHandlerStmt(ctxName string, annotationName string) ast.Stmt {
	return &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{
				ast.NewIdent("before"),
				ast.NewIdent("exists"),
			},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{
				&ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   ast.NewIdent("gdi"),
						Sel: ast.NewIdent("GetBeforeAnnotationHandler"),
					},
					Args: []ast.Expr{
						&ast.BasicLit{
							Kind:  token.STRING,
							Value: fmt.Sprintf(`"%s"`, annotationName),
						},
					},
				},
			},
		},
		Cond: ast.NewIdent("exists"),
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ExprStmt{
					X: &ast.CallExpr{
						Fun:  ast.NewIdent("before"),
						Args: []ast.Expr{ast.NewIdent(ctxName)},
					},
				},
			},
		},
	}
}

// createAfterHandlerStmt 创建后置处理
func createAfterHandlerStmt(ctxName string, annotationName string) ast.Stmt {
	return &ast.IfStmt{
		Init: &ast.AssignStmt{
			Lhs: []ast.Expr{
				ast.NewIdent("after"),
				ast.NewIdent("exists"),
			},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{
				&ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   ast.NewIdent("gdi"),
						Sel: ast.NewIdent("GetAfterAnnotationHandler"),
					},
					Args: []ast.Expr{
						&ast.BasicLit{
							Kind:  token.STRING,
							Value: fmt.Sprintf(`"%s"`, annotationName),
						},
					},
				},
			},
		},
		Cond: ast.NewIdent("exists"),
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ExprStmt{
					X: &ast.CallExpr{
						Fun:  ast.NewIdent("after"),
						Args: []ast.Expr{ast.NewIdent(ctxName)},
					},
				},
			},
		},
	}
}

// createSetEndTimeStmt 设置结束时间
func createSetEndTimeStmt(ctxName string) ast.Stmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{
			&ast.SelectorExpr{
				X:   ast.NewIdent(ctxName),
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
