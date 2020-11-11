// 包heap为实现heap.interface的任何类型提供堆操作。堆是一棵树，其属性是每个节点都是其子树中的最小值节点。树中的最小元素是索引0处的根。
// 堆是实现优先队列的一种常用方法。要构建一个优先级队列，实现以(负)优先级作为Less方法的顺序的堆接口，因此Push添加元素，而Pop从队列中删除最高优先级的元素。

package heap

import "sort"

// 接口类型描述使用此包中的例程的类型的需求。任何实现它的类型都可以作为最小堆使用以下不变量(在Init被调用后建立，或者如果数据是空的或排序):
//
//	!h.Less(j, i) for 0 <= i < h.Len() and 2*i+1 <= j <= 2*i+2 and j < h.Len()
//
// 注意，这个接口中的Push和Pop是为了调用包堆的实现。要从堆中添加和删除内容，请使用heap.Push和heap.Pop。
type Interface interface {
	sort.Interface // 扩展排序接口
	Push(x interface{}) // 添加x作为元素Len()
	Pop() interface{}   // 移除并返回元素Len()-1。
}

// Init建立这个包中的其他例程所需的堆不变量。Init对于堆不变量是等幂的，并且可以在堆不变量无效时调用。复杂度是O(n)其中n = h.Len()
func Init(h Interface) {
	// 构建堆
	n := h.Len()
	for i := n/2 - 1; i >= 0; i-- {
		down(h, i, n)
	}
}

// Push 将元素x添加到堆上
// 复杂度为 O(log n)，其中 n = h.Len()
func Push(h Interface, x interface{}) {
	h.Push(x)
	up(h, h.Len()-1)
}

// Pop从堆中移除并返回最小元素(根据Less)。复杂度是O(log n)其中n = h.Len()Pop相当于Remove(h, 0)。
func Pop(h Interface) interface{} {
	n := h.Len() - 1
	h.Swap(0, n)
	down(h, 0, n)
	return h.Pop()
}

// Remove移除并返回堆中索引i处的元素。复杂度是O(log n)其中n = h.Len()
func Remove(h Interface, i int) interface{} {
	n := h.Len() - 1
	if n != i {
		h.Swap(i, n)
		if !down(h, i, n) {
			up(h, i)
		}
	}
	return h.Pop()
}

// Fix在索引i处的元素改变其值后重新建立堆排序。在索引i处更改元素的值，然后调用Fix，这与在推入新值之后调用Remove(h, i)相等，但代价要低一些。复杂度是O(log n)其中n = h.Len()
func Fix(h Interface, i int) {
	if !down(h, i, h.Len()) {
		up(h, i)
	}
}

func up(h Interface, j int) {
	for {
		i := (j - 1) / 2 // parent
		if i == j || !h.Less(j, i) {
			break
		}
		h.Swap(i, j)
		j = i
	}
}

func down(h Interface, i0, n int) bool {
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 { // j1 < 0 after int overflow
			break
		}
		j := j1 // left child
		if j2 := j1 + 1; j2 < n && h.Less(j2, j1) {
			j = j2 // = 2*i + 2  // right child
		}
		if !h.Less(j, i) {
			break
		}
		h.Swap(i, j)
		i = j
	}
	return i > i0
}
