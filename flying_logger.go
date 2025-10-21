package flying

import "fmt"

type defaultLogger struct{}

func (d *defaultLogger) Infof(format string, args ...any)  { fmt.Printf("[I] "+format, args...) }
func (d *defaultLogger) Warnf(format string, args ...any)  { fmt.Printf("[W] "+format, args...) }
func (d *defaultLogger) Errorf(format string, args ...any) { fmt.Printf("[E] "+format, args...) }