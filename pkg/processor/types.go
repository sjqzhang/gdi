package processor

// Annotation 表示一个注解及其参数
type Annotation struct {
	Name   string            // 注解名称
	Params map[string]string // 注解参数
}
