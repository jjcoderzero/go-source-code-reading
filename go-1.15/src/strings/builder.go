package strings

import (
	"unicode/utf8"
	"unsafe"
)

// Builder用于使用Write方法有效的构建字符串。它最小化内存复制。零值已经可以使用了。不要复制一个非零构建器。
type Builder struct {
	addr *Builder // 在接收端，通过值检测拷贝
	buf  []byte
}

// noescape在转义分析中隐藏指针。noescape是恒等函数，但escape分析认为输出并不依赖于输入。noescape是内联的，目前编译到零指令。
// USE CAREFULLY!
//go:nosplit
//go:nocheckptr
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}

func (b *Builder) copyCheck() {
	if b.addr == nil {
		// 这个hack的工作围绕着一个失败的Go的转义分析导致b转义和堆分配。
		// just "b.addr = b".
		b.addr = (*Builder)(noescape(unsafe.Pointer(b)))
	} else if b.addr != b {
		panic("strings: illegal use of non-zero Builder copied by value")
	}
}

// String返回累积的字符串。
func (b *Builder) String() string {
	return *(*string)(unsafe.Pointer(&b.buf))
}

// Len返回累积的字节数;b.Len () = = len (b.String())。
func (b *Builder) Len() int { return len(b.buf) }

// Cap返回构建器的底层字节片的容量。它是为正在构建的字符串分配的总空间，包括已经写入的任何字节。
func (b *Builder) Cap() int { return cap(b.buf) }

// Reset将构建器重置为空。
func (b *Builder) Reset() {
	b.addr = nil
	b.buf = nil
}

// grow将缓冲区复制到一个新的、更大的缓冲区，以便在len(b.buf)之外至少有n个字节的容量。
func (b *Builder) grow(n int) {
	buf := make([]byte, len(b.buf), 2*cap(b.buf)+n)
	copy(buf, b.buf)
	b.buf = buf
}

// Grow在必要时增加b的容量，以保证另外n个字节的空间。在Grow(n)之后，至少有n个字节可以被写入b，而无需进行另一次分配。如果n是负的，就会变得恐慌。
func (b *Builder) Grow(n int) {
	b.copyCheck()
	if n < 0 {
		panic("strings.Builder.Grow: negative count")
	}
	if cap(b.buf)-len(b.buf) < n {
		b.grow(n)
	}
}

// Write将p的内容追加到b的缓冲区。写总是返回len(p)， nil。
func (b *Builder) Write(p []byte) (int, error) {
	b.copyCheck()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

// WriteByte将字节c追加到b的缓冲区。返回的错误总是nil。
func (b *Builder) WriteByte(c byte) error {
	b.copyCheck()
	b.buf = append(b.buf, c)
	return nil
}

// WriteRune将Unicode码点r的UTF-8编码追加到b的缓冲区。它返回r的长度和nil错误。
func (b *Builder) WriteRune(r rune) (int, error) {
	b.copyCheck()
	if r < utf8.RuneSelf {
		b.buf = append(b.buf, byte(r))
		return 1, nil
	}
	l := len(b.buf)
	if cap(b.buf)-l < utf8.UTFMax {
		b.grow(utf8.UTFMax)
	}
	n := utf8.EncodeRune(b.buf[l:l+utf8.UTFMax], r)
	b.buf = b.buf[:l+n]
	return n, nil
}

// WriteString将s的内容追加到b的缓冲区。它返回s的长度和nil错误。
func (b *Builder) WriteString(s string) (int, error) {
	b.copyCheck()
	b.buf = append(b.buf, s...)
	return len(s), nil
}
