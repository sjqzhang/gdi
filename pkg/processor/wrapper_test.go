package processor

import (
	"testing"
)

func TestWrapFunction(t *testing.T) {
	src_func := `
//go:gdi log 使用内置的日志注解
//go:gdi timer(threshold="200ms") 使用内置的计时器注解
//go:gdi transaction 使用自定义的事务注解
func (s *UserService) CreateUser(name string) string {

	return "user:" + name
}
`
	target_func := `func (s *UserService) CreateUser(name string) string {
    // log装饰器（最外层）
    ctx_0 := &gdi.Context{
        Method:     "CreateUser",
        Args:       []interface{}{name},
        Properties: make(map[string]interface{}),
        StartTime:  time.Now(),
    }

    // log装饰器的前置处理
    if before, exists := gdi.GetBeforeAnnotationHandler("log"); exists {
        before(ctx_0)
    }

    // 执行timer装饰器（中间层）
    result := func() string {
        ctx_1 := &gdi.Context{
            Method:     "CreateUser",
            Args:       []interface{}{name},
            Properties: make(map[string]interface{}),
            StartTime:  time.Now(),
        }

        // 解析timer的参数
        ctx_1.Properties["threshold"] = time.ParseDuration("200ms")

        // timer装饰器的前置处理
        if before, exists := gdi.GetBeforeAnnotationHandler("timer"); exists {
            before(ctx_1)
        }

        // 执行transaction装饰器（最内层）
        result := func() string {
            ctx_2 := &gdi.Context{
                Method:     "CreateUser",
                Args:       []interface{}{name},
                Properties: make(map[string]interface{}),
                StartTime:  time.Now(),
            }

            // transaction装饰器的前置处理
            if before, exists := gdi.GetBeforeAnnotationHandler("transaction"); exists {
                before(ctx_2)
            }

            // 执行原始方法
            result := func() string {
                return "user:" + name
            }()

            // 记录transaction装饰器的结束时间
            ctx_2.EndTime = time.Now()
            ctx_2.Returns = []interface{}{result}

            // transaction装饰器的后置处理
            if after, exists := gdi.GetAfterAnnotationHandler("transaction"); exists {
                after(ctx_2)
            }

            return result
        }()

        // 记录timer装饰器的结束时间
        ctx_1.EndTime = time.Now()
        ctx_1.Returns = []interface{}{result}

        // timer装饰器的后置处理
        if after, exists := gdi.GetAfterAnnotationHandler("timer"); exists {
            after(ctx_1)
        }

        return result
    }()

    // 记录log装饰器的结束时间
    ctx_0.EndTime = time.Now()
    ctx_0.Returns = []interface{}{result}

    // log装饰器的后置处理
    if after, exists := gdi.GetAfterAnnotationHandler("log"); exists {
        after(ctx_0)
    }

    return result
}
`

	_ = src_func
	_ = target_func

}
