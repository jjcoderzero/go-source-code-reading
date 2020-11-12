package sync

import (
	"runtime"
	"std/internal/race"
	"sync/atomic"
	"unsafe"
)

// Pool是一组可以单独保存和检索的临时对象。
// 存储在池中的任何项都可以在任何时候自动删除，而不需要通知。如果在这种情况发生时池持有唯一的引用，则项目可能被释放。
// 一个池是安全的，由多个goroutines同时使用。
// 池的目的是缓存已分配但未使用的项目，以便以后重用，减轻垃圾收集器的压力。也就是说，它可以很容易地构建高效的、线程安全的空闲列表。
// 但是，它并不适用于所有空闲列表。池的适当使用是管理一组临时项，这些临时项在包的并发独立客户端之间以静默方式共享，并可能由这些客户端重用。
// Pool提供了一种在许多客户端分摊分配开销的方法。良好使用池的一个例子是fmt包，它维护临时输出缓冲区的动态大小的存储。
// 商店在负载下缩放(当许多goroutines正在积极打印)，并在静止时收缩。另一方面，作为短期存在对象的一部分维护的空闲列表不适用于池，因为开销在该场景中不能很好地摊销。
// 让这些对象实现它们自己的空闲列表会更有效。
//
// 池在第一次使用后不能复制。
type Pool struct {
	noCopy noCopy

	local     unsafe.Pointer // 本地固定大小的per-P池，实际类型为[P]poolLocal
	localSize uintptr        // 本地数组的大小

	victim     unsafe.Pointer // 前一个周期的局部
	victimSize uintptr        // victims数组的大小

	New func() interface{} // 当Get返回nil时，New可选地指定一个函数来生成一个值。不能在调用Get时同时更改它。
}

// 本地per-P池附录。
type poolLocalInternal struct {
	private interface{} // 只能被各自的P所使用。
	shared  poolChain   // Local P can pushHead/popHead; any P can popTail.
}

type poolLocal struct {
	poolLocalInternal

	// 防止在广泛使用的平台上的错误共享128 mod(高速缓存线大小)= 0。
	pad [128 - unsafe.Sizeof(poolLocalInternal{})%128]byte
}

// from runtime
func fastrand() uint32

var poolRaceHash [128]uint64

// poolRaceAddr返回一个地址，用作竞争检测器逻辑的同步点。我们不直接使用存储在x中的实际指针，以免与该地址上的其他同步发生冲突。相反，我们对指针进行散列以获得到poolRaceHash的索引
func poolRaceAddr(x interface{}) unsafe.Pointer {
	ptr := uintptr((*[2]unsafe.Pointer)(unsafe.Pointer(&x))[1])
	h := uint32((uint64(uint32(ptr)) * 0x85ebca6b) >> 16)
	return unsafe.Pointer(&poolRaceHash[h%uint32(len(poolRaceHash))])
}

// Put adds x to the pool.
func (p *Pool) Put(x interface{}) {
	if x == nil {
		return
	}
	if race.Enabled {
		if fastrand()%4 == 0 {
			// 随机把x丢在地板上。
			return
		}
		race.ReleaseMerge(poolRaceAddr(x))
		race.Disable()
	}
	l, _ := p.pin()
	if l.private == nil {
		l.private = x
		x = nil
	}
	if x != nil {
		l.shared.pushHead(x)
	}
	runtime_procUnpin()
	if race.Enabled {
		race.Enable()
	}
}

// Get从池中选择一个任意项，将其从池中移除，并将其返回给调用者。Get可能选择忽略池并将其视为空。调用者不应该假定传递给Put的值和Get返回的值之间有任何关系。
// 如果Get返回nil，而p.New是非nil，那么Get返回调用p.New的结果。
func (p *Pool) Get() interface{} {
	if race.Enabled {
		race.Disable()
	}
	l, pid := p.pin()
	x := l.private
	l.private = nil
	if x == nil {
		// Try to pop the head of the local shard. We prefer
		// the head over the tail for temporal locality of
		// reuse.
		x, _ = l.shared.popHead()
		if x == nil {
			x = p.getSlow(pid)
		}
	}
	runtime_procUnpin()
	if race.Enabled {
		race.Enable()
		if x != nil {
			race.Acquire(poolRaceAddr(x))
		}
	}
	if x == nil && p.New != nil {
		x = p.New()
	}
	return x
}

func (p *Pool) getSlow(pid int) interface{} {
	// See the comment in pin regarding ordering of the loads.
	size := atomic.LoadUintptr(&p.localSize) // load-acquire
	locals := p.local                        // load-consume
	// Try to steal one element from other procs.
	for i := 0; i < int(size); i++ {
		l := indexLocal(locals, (pid+i+1)%int(size))
		if x, _ := l.shared.popTail(); x != nil {
			return x
		}
	}

	// Try the victim cache. We do this after attempting to steal
	// from all primary caches because we want objects in the
	// victim cache to age out if at all possible.
	size = atomic.LoadUintptr(&p.victimSize)
	if uintptr(pid) >= size {
		return nil
	}
	locals = p.victim
	l := indexLocal(locals, pid)
	if x := l.private; x != nil {
		l.private = nil
		return x
	}
	for i := 0; i < int(size); i++ {
		l := indexLocal(locals, (pid+i)%int(size))
		if x, _ := l.shared.popTail(); x != nil {
			return x
		}
	}

	// Mark the victim cache as empty for future gets don't bother
	// with it.
	atomic.StoreUintptr(&p.victimSize, 0)

	return nil
}

// pin将当前goroutine引到P，禁用抢占并返回P和P id的poolLocal池。调用者必须调用runtime_procUnpin()。
func (p *Pool) pin() (*poolLocal, int) {
	pid := runtime_procPin()
	// In pinSlow we store to local and then to localSize, here we load in opposite order.
	// Since we've disabled preemption, GC cannot happen in between.
	// Thus here we must observe local at least as large localSize.
	// We can observe a newer/larger local, it is fine (we must observe its zero-initialized-ness).
	s := atomic.LoadUintptr(&p.localSize) // load-acquire
	l := p.local                          // load-consume
	if uintptr(pid) < s {
		return indexLocal(l, pid), pid
	}
	return p.pinSlow()
}

func (p *Pool) pinSlow() (*poolLocal, int) {
	// Retry under the mutex.
	// Can not lock the mutex while pinned.
	runtime_procUnpin()
	allPoolsMu.Lock()
	defer allPoolsMu.Unlock()
	pid := runtime_procPin()
	// poolCleanup 不会被调用但我们被固定时。
	s := p.localSize
	l := p.local
	if uintptr(pid) < s {
		return indexLocal(l, pid), pid
	}
	if p.local == nil {
		allPools = append(allPools, p)
	}
	// 如果GOMAXPROCS在不同的GCs之间发生变化，我们将重新分配数组并丢失原来的那个。
	size := runtime.GOMAXPROCS(0)
	local := make([]poolLocal, size)
	atomic.StorePointer(&p.local, unsafe.Pointer(&local[0])) // store-release
	atomic.StoreUintptr(&p.localSize, uintptr(size))         // store-release
	return &local[pid], pid
}

func poolCleanup() {
	// This function is called with the world stopped, at the beginning of a garbage collection.
	// It must not allocate and probably should not call any runtime functions.

	// Because the world is stopped, no pool user can be in a
	// pinned section (in effect, this has all Ps pinned).

	// Drop victim caches from all pools.
	for _, p := range oldPools {
		p.victim = nil
		p.victimSize = 0
	}

	// Move primary cache to victim cache.
	for _, p := range allPools {
		p.victim = p.local
		p.victimSize = p.localSize
		p.local = nil
		p.localSize = 0
	}

	// 具有非空主缓存的池现在具有非空的victim缓存，并且没有池具有主缓存。
	oldPools, allPools = allPools, nil
}

var (
	allPoolsMu Mutex

	allPools []*Pool // allPools是一组具有非空主缓存的池。由1)allPoolsMu和pinning或2)STW保护。

	oldPools []*Pool // oldPools是一组可能具有非空victim缓存的池。受STW保护。
)

func init() {
	runtime_registerPoolCleanup(poolCleanup)
}

func indexLocal(l unsafe.Pointer, i int) *poolLocal {
	lp := unsafe.Pointer(uintptr(l) + uintptr(i)*unsafe.Sizeof(poolLocal{}))
	return (*poolLocal)(lp)
}

// Implemented in runtime.
func runtime_registerPoolCleanup(cleanup func())
func runtime_procPin() int
func runtime_procUnpin()
