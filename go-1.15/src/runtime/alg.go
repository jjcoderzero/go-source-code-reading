package runtime

import (
	"std/internal/cpu"
	"std/runtime/internal/sys"
	"unsafe"
)

const (
	c0 = uintptr((8-sys.PtrSize)/4*2860486313 + (sys.PtrSize-4)/4*33054211828000289)
	c1 = uintptr((8-sys.PtrSize)/4*3267000013 + (sys.PtrSize-4)/4*23344194077549503)
)

// 类型算法-已知的编译器
const (
	alg_NOEQ = iota
	alg_MEM0
	alg_MEM8
	alg_MEM16
	alg_MEM32
	alg_MEM64
	alg_MEM128
	alg_STRING
	alg_INTER
	alg_NILINTER
	alg_FLOAT32
	alg_FLOAT64
	alg_CPLX64
	alg_CPLX128
	alg_max
)

func memhash0(p unsafe.Pointer, h uintptr) uintptr {
	return h
}

func memhash8(p unsafe.Pointer, h uintptr) uintptr {
	return memhash(p, h, 1)
}

func memhash16(p unsafe.Pointer, h uintptr) uintptr {
	return memhash(p, h, 2)
}

func memhash128(p unsafe.Pointer, h uintptr) uintptr {
	return memhash(p, h, 16)
}

//go:nosplit
func memhash_varlen(p unsafe.Pointer, h uintptr) uintptr {
	ptr := getclosureptr()
	size := *(*uintptr)(unsafe.Pointer(ptr + unsafe.Sizeof(h)))
	return memhash(p, h, size)
}

// 运行时变量，检查我们运行的处理器是否真正支持基于aes的哈希实现所使用的指令。
var useAeshash bool

// in asm_*.s
func memhash(p unsafe.Pointer, h, s uintptr) uintptr
func memhash32(p unsafe.Pointer, h uintptr) uintptr
func memhash64(p unsafe.Pointer, h uintptr) uintptr
func strhash(p unsafe.Pointer, h uintptr) uintptr

func strhashFallback(a unsafe.Pointer, h uintptr) uintptr {
	x := (*stringStruct)(a)
	return memhashFallback(x.str, h, uintptr(x.len))
}

// 注意:因为NaN != NaN，一个映射可以包含任意数量的(大部分是无用的)以NaN为键的条目。为了避免长散列链，我们为NaN分配了一个随机数作为散列值。

func f32hash(p unsafe.Pointer, h uintptr) uintptr {
	f := *(*float32)(p)
	switch {
	case f == 0:
		return c1 * (c0 ^ h) // +0, -0
	case f != f:
		return c1 * (c0 ^ h ^ uintptr(fastrand())) // any kind of NaN
	default:
		return memhash(p, h, 4)
	}
}

func f64hash(p unsafe.Pointer, h uintptr) uintptr {
	f := *(*float64)(p)
	switch {
	case f == 0:
		return c1 * (c0 ^ h) // +0, -0
	case f != f:
		return c1 * (c0 ^ h ^ uintptr(fastrand())) // any kind of NaN
	default:
		return memhash(p, h, 8)
	}
}

func c64hash(p unsafe.Pointer, h uintptr) uintptr {
	x := (*[2]float32)(p)
	return f32hash(unsafe.Pointer(&x[1]), f32hash(unsafe.Pointer(&x[0]), h))
}

func c128hash(p unsafe.Pointer, h uintptr) uintptr {
	x := (*[2]float64)(p)
	return f64hash(unsafe.Pointer(&x[1]), f64hash(unsafe.Pointer(&x[0]), h))
}

func interhash(p unsafe.Pointer, h uintptr) uintptr {
	a := (*iface)(p)
	tab := a.tab
	if tab == nil {
		return h
	}
	t := tab._type
	if t.equal == nil {
		// 检查hashability。我们可以在typehash中做这个检查，但是我们想报告错误文本中最上面的类型(例如，在具有slice类型字段的struct中，我们想报告结构，而不是slice)。
		panic(errorString("hash of unhashable type " + t.string()))
	}
	if isDirectIface(t) {
		return c1 * typehash(t, unsafe.Pointer(&a.data), h^c0)
	} else {
		return c1 * typehash(t, a.data, h^c0)
	}
}

func nilinterhash(p unsafe.Pointer, h uintptr) uintptr {
	a := (*eface)(p)
	t := a._type
	if t == nil {
		return h
	}
	if t.equal == nil {
		// See comment in interhash above.
		panic(errorString("hash of unhashable type " + t.string()))
	}
	if isDirectIface(t) {
		return c1 * typehash(t, unsafe.Pointer(&a.data), h^c0)
	} else {
		return c1 * typehash(t, a.data, h^c0)
	}
}

// typehash计算在地址p.h处类型为t的对象的哈希值。这个函数很少使用。大多数映射要么用于哈希固定函数(例如f32hash)，要么用于编译器生成的函数(例如struct {x, y string})。
// 这种实现速度较慢，但更通用，用于对接口类型进行散列(从interhash或nilinterhash中调用，见上)或在反射生成的映射中进行散列。MapOf (reflect_typehash,下文)。
// 注意:这个函数必须与编译器生成的函数完全匹配。
func typehash(t *_type, p unsafe.Pointer, h uintptr) uintptr {
	if t.tflag&tflagRegularMemory != 0 {
		// 特殊处理ptr通径
		switch t.size {
		case 4:
			return memhash32(p, h)
		case 8:
			return memhash64(p, h)
		default:
			return memhash(p, h, t.size)
		}
	}
	switch t.kind & kindMask {
	case kindFloat32:
		return f32hash(p, h)
	case kindFloat64:
		return f64hash(p, h)
	case kindComplex64:
		return c64hash(p, h)
	case kindComplex128:
		return c128hash(p, h)
	case kindString:
		return strhash(p, h)
	case kindInterface:
		i := (*interfacetype)(unsafe.Pointer(t))
		if len(i.mhdr) == 0 {
			return nilinterhash(p, h)
		}
		return interhash(p, h)
	case kindArray:
		a := (*arraytype)(unsafe.Pointer(t))
		for i := uintptr(0); i < a.len; i++ {
			h = typehash(a.elem, add(p, i*a.elem.size), h)
		}
		return h
	case kindStruct:
		s := (*structtype)(unsafe.Pointer(t))
		memStart := uintptr(0)
		memEnd := uintptr(0)
		for _, f := range s.fields {
			if memEnd > memStart && (f.name.isBlank() || f.offset() != memEnd || f.typ.tflag&tflagRegularMemory == 0) {
				// flush any pending regular memory hashing
				h = memhash(add(p, memStart), h, memEnd-memStart)
				memStart = memEnd
			}
			if f.name.isBlank() {
				continue
			}
			if f.typ.tflag&tflagRegularMemory == 0 {
				h = typehash(f.typ, add(p, f.offset()), h)
				continue
			}
			if memStart == memEnd {
				memStart = f.offset()
			}
			memEnd = f.offset() + f.typ.size
		}
		if memEnd > memStart {
			h = memhash(add(p, memStart), h, memEnd-memStart)
		}
		return h
	default:
		// Should never happen, as typehash should only be called
		// with comparable types.
		panic(errorString("hash of unhashable type " + t.string()))
	}
}

//go:linkname reflect_typehash reflect.typehash
func reflect_typehash(t *_type, p unsafe.Pointer, h uintptr) uintptr {
	return typehash(t, p, h)
}

func memequal0(p, q unsafe.Pointer) bool {
	return true
}
func memequal8(p, q unsafe.Pointer) bool {
	return *(*int8)(p) == *(*int8)(q)
}
func memequal16(p, q unsafe.Pointer) bool {
	return *(*int16)(p) == *(*int16)(q)
}
func memequal32(p, q unsafe.Pointer) bool {
	return *(*int32)(p) == *(*int32)(q)
}
func memequal64(p, q unsafe.Pointer) bool {
	return *(*int64)(p) == *(*int64)(q)
}
func memequal128(p, q unsafe.Pointer) bool {
	return *(*[2]int64)(p) == *(*[2]int64)(q)
}
func f32equal(p, q unsafe.Pointer) bool {
	return *(*float32)(p) == *(*float32)(q)
}
func f64equal(p, q unsafe.Pointer) bool {
	return *(*float64)(p) == *(*float64)(q)
}
func c64equal(p, q unsafe.Pointer) bool {
	return *(*complex64)(p) == *(*complex64)(q)
}
func c128equal(p, q unsafe.Pointer) bool {
	return *(*complex128)(p) == *(*complex128)(q)
}
func strequal(p, q unsafe.Pointer) bool {
	return *(*string)(p) == *(*string)(q)
}
func interequal(p, q unsafe.Pointer) bool {
	x := *(*iface)(p)
	y := *(*iface)(q)
	return x.tab == y.tab && ifaceeq(x.tab, x.data, y.data)
}
func nilinterequal(p, q unsafe.Pointer) bool {
	x := *(*eface)(p)
	y := *(*eface)(q)
	return x._type == y._type && efaceeq(x._type, x.data, y.data)
}
func efaceeq(t *_type, x, y unsafe.Pointer) bool {
	if t == nil {
		return true
	}
	eq := t.equal
	if eq == nil {
		panic(errorString("comparing uncomparable type " + t.string()))
	}
	if isDirectIface(t) {
		// Direct interface types are ptr, chan, map, func, and single-element structs/arrays thereof.
		// Maps and funcs are not comparable, so they can't reach here.
		// Ptrs, chans, and single-element items can be compared directly using ==.
		return x == y
	}
	return eq(x, y)
}
func ifaceeq(tab *itab, x, y unsafe.Pointer) bool {
	if tab == nil {
		return true
	}
	t := tab._type
	eq := t.equal
	if eq == nil {
		panic(errorString("comparing uncomparable type " + t.string()))
	}
	if isDirectIface(t) {
		// See comment in efaceeq.
		return x == y
	}
	return eq(x, y)
}

// Testing adapters for hash quality tests (see hash_test.go)
func stringHash(s string, seed uintptr) uintptr {
	return strhash(noescape(unsafe.Pointer(&s)), seed)
}

func bytesHash(b []byte, seed uintptr) uintptr {
	s := (*slice)(unsafe.Pointer(&b))
	return memhash(s.array, seed, uintptr(s.len))
}

func int32Hash(i uint32, seed uintptr) uintptr {
	return memhash32(noescape(unsafe.Pointer(&i)), seed)
}

func int64Hash(i uint64, seed uintptr) uintptr {
	return memhash64(noescape(unsafe.Pointer(&i)), seed)
}

func efaceHash(i interface{}, seed uintptr) uintptr {
	return nilinterhash(noescape(unsafe.Pointer(&i)), seed)
}

func ifaceHash(i interface {
	F()
}, seed uintptr) uintptr {
	return interhash(noescape(unsafe.Pointer(&i)), seed)
}

const hashRandomBytes = sys.PtrSize / 4 * 64

var aeskeysched [hashRandomBytes]byte // 用于asm_ {386 amd64, arm64}。将散列函数作为种子

var hashkey [4]uintptr // 用于hash{32,64}.给哈希函数添加种子

func alginit() {
	// 如果有需要的指令，则安装AES哈希算法。
	if (GOARCH == "386" || GOARCH == "amd64") &&
		cpu.X86.HasAES && // AESENC
		cpu.X86.HasSSSE3 && // PSHUFB
		cpu.X86.HasSSE41 { // PINSR{D,Q}
		initAlgAES()
		return
	}
	if GOARCH == "arm64" && cpu.ARM64.HasAES {
		initAlgAES()
		return
	}
	getRandomData((*[len(hashkey) * sys.PtrSize]byte)(unsafe.Pointer(&hashkey))[:])
	hashkey[0] |= 1 // 确保这些数字是奇数
	hashkey[1] |= 1
	hashkey[2] |= 1
	hashkey[3] |= 1
}

func initAlgAES() {
	useAeshash = true
	// 初始化随机数据，所以哈希冲突将很难设计。
	getRandomData(aeskeysched[:])
}

// Note: These routines perform the read with a native endianness.
func readUnaligned32(p unsafe.Pointer) uint32 {
	q := (*[4]byte)(p)
	if sys.BigEndian {
		return uint32(q[3]) | uint32(q[2])<<8 | uint32(q[1])<<16 | uint32(q[0])<<24
	}
	return uint32(q[0]) | uint32(q[1])<<8 | uint32(q[2])<<16 | uint32(q[3])<<24
}

func readUnaligned64(p unsafe.Pointer) uint64 {
	q := (*[8]byte)(p)
	if sys.BigEndian {
		return uint64(q[7]) | uint64(q[6])<<8 | uint64(q[5])<<16 | uint64(q[4])<<24 |
			uint64(q[3])<<32 | uint64(q[2])<<40 | uint64(q[1])<<48 | uint64(q[0])<<56
	}
	return uint64(q[0]) | uint64(q[1])<<8 | uint64(q[2])<<16 | uint64(q[3])<<24 | uint64(q[4])<<32 | uint64(q[5])<<40 | uint64(q[6])<<48 | uint64(q[7])<<56
}
