package fmt

import "errors"

// Errorf根据格式说明符进行格式化，并将字符串作为满足错误的值返回。如果格式说明符包含带有错误操作数的%w谓词，则返回的错误将实现返回该操作数的解包装方法。
// 包含一个以上的%w谓词或为其提供一个没有实现错误接口的操作数是无效的。动词%w在其他情况下是%v的同义词。
func Errorf(format string, a ...interface{}) error {
	p := newPrinter()
	p.wrapErrs = true
	p.doPrintf(format, a)
	s := string(p.buf)
	var err error
	if p.wrappedErr == nil {
		err = errors.New(s)
	} else {
		err = &wrapError{s, p.wrappedErr}
	}
	p.free()
	return err
}

type wrapError struct {
	msg string
	err error
}

func (e *wrapError) Error() string {
	return e.msg
}

func (e *wrapError) Unwrap() error {
	return e.err
}
