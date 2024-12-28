package processor

import (
	"fmt"
	"os"
)

var (
	debug   = os.Getenv("GDI_DEBUG") == "1"
	logFile = os.Getenv("GDI_LOG")
)

// debugf 输出调试信息
func debugf(format string, args ...interface{}) {
	if debug {
		msg := fmt.Sprintf("[GDI_DEBUG] "+format+"\n", args...)
		fmt.Print(msg)
		if logFile != "" {
			f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				defer f.Close()
				f.WriteString(msg)
			}
		}
	}
}
