package gdi

import (
	"fmt"
	"time"
)

// Context 注解上下文
type Context struct {
	Target     interface{}            // 目标对象
	Method     string                 // 方法名
	Args       []interface{}          // 参数
	Returns    []interface{}          // 返回值
	StartTime  time.Time              // 开始时间
	EndTime    time.Time              // 结束时间
	Properties map[string]interface{} // 自定义属性
}

// AnnotationFunc 注解处理函数类型
type AnnotationFunc func(ctx *Context) error

var (
	beforeHandlers = make(map[string]AnnotationFunc)
	afterHandlers  = make(map[string]AnnotationFunc)
)

// RegisterAnnotation 注册自定义注解
func RegisterAnnotation(name string, before, after AnnotationFunc) {
	if before != nil {
		beforeHandlers[name] = before
	}
	if after != nil {
		afterHandlers[name] = after
	}
}

// 内置的日志注解处理函数
func init() {
	// 注册日志注解
	RegisterAnnotation("log",
		func(ctx *Context) error {
			fmt.Printf("[%s] Entering method: %s\n", time.Now().Format("2006-01-02 15:04:05"), ctx.Method)
			fmt.Printf("Arguments: %v\n", ctx.Args)
			return nil
		},
		func(ctx *Context) error {
			duration := ctx.EndTime.Sub(ctx.StartTime)
			fmt.Printf("[%s] Exiting method: %s (duration: %v)\n",
				time.Now().Format("2006-01-02 15:04:05"),
				ctx.Method,
				duration)
			fmt.Printf("Returns: %v\n", ctx.Returns)
			return nil
		},
	)

	// 注册计时器注解
	RegisterAnnotation("timer", nil,
		func(ctx *Context) error {
			duration := ctx.EndTime.Sub(ctx.StartTime)
			if duration > 100*time.Millisecond {
				fmt.Printf("[SLOW] Method %s took %v to execute\n", ctx.Method, duration)
			}
			return nil
		},
	)
}
