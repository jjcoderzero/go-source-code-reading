// 包list实现了一个双重链表。
//
// To iterate over a list (where l is a *List):
//	for e := l.Front(); e != nil; e = e.Next() {
//		// do something with e.Value
//	}
//
package list

// Element是链表中的元素。
type Element struct {
	// 双链元素列表中的下一个和上一个指针。为了简化实现，在内部将列表l实现为一个环，这样&l.root既是最后一个列表元素(l. back())的下一个元素，也是第一个列表元素(l. front())的前一个元素。
	next, prev *Element //

	list *List // 此元素所属的列表。

	Value interface{} // 与此元素一起存储的值。
}

// Next返回下一个列表元素或nil。
func (e *Element) Next() *Element {
	if p := e.next; e.list != nil && p != &e.list.root {
		return p
	}
	return nil
}

// Prev返回之前的列表元素或nil。
func (e *Element) Prev() *Element {
	if p := e.prev; e.list != nil && p != &e.list.root {
		return p
	}
	return nil
}

// List表示双链表。List的零值是准备使用的空列表。
type List struct {
	root Element // sentinel列表元素，仅使用&root、root.next和root.prev
	len  int     // 不包括(这个)标记元素的当前列表长度
}

// Init初始化或清除列表l。
func (l *List) Init() *List {
	l.root.next = &l.root
	l.root.prev = &l.root
	l.len = 0
	return l
}

// New返回一个初始化的列表。
func New() *List { return new(List).Init() }

// Len返回列表l的元素数量，复杂度为O(1)。
func (l *List) Len() int { return l.len }

// Front返回列表l的第一个元素，如果列表为空，则返回nil。
func (l *List) Front() *Element {
	if l.len == 0 {
		return nil
	}
	return l.root.next
}

// Back返回列表l的最后一个元素，如果列表为空，则返回nil。
func (l *List) Back() *Element {
	if l.len == 0 {
		return nil
	}
	return l.root.prev
}

// lazyInit惰性地初始化一个零列表值。
func (l *List) lazyInit() {
	if l.root.next == nil {
		l.Init()
	}
}

// insert在at后面插入e，对l.len进行增量，然后返回e。
func (l *List) insert(e, at *Element) *Element {
	e.prev = at
	e.next = at.next
	e.prev.next = e
	e.next.prev = e
	e.list = l
	l.len++
	return e
}

// insertValue是一个方便的insert(&Element{Value: v}， at)包装器。
func (l *List) insertValue(v interface{}, at *Element) *Element {
	return l.insert(&Element{Value: v}, at)
}

// remove从它的列表中移除e，递减l.len，并返回e。
func (l *List) remove(e *Element) *Element {
	e.prev.next = e.next
	e.next.prev = e.prev
	e.next = nil // 避免内存泄漏
	e.prev = nil // 避免内存泄漏
	e.list = nil
	l.len--
	return e
}

// move将e移到at的旁边，然后返回e。
func (l *List) move(e, at *Element) *Element {
	if e == at {
		return e
	}
	e.prev.next = e.next
	e.next.prev = e.prev

	e.prev = at
	e.next = at.next
	e.prev.next = e
	e.next.prev = e

	return e
}

// 如果e是列表l的一个元素，那么Remove将e从l中移除。它返回元素值e. value。元素不能为nil。
func (l *List) Remove(e *Element) interface{} {
	if e.list == l {
		// 如果e.list == l，则l必须在e插入l时已经初始化，或者l == nil (e是一个零元素)，并且l.remove将崩溃
		l.remove(e)
	}
	return e.Value
}

// PushFront在列表l的前面插入一个新元素e，并返回e。
func (l *List) PushFront(v interface{}) *Element {
	l.lazyInit()
	return l.insertValue(v, &l.root)
}

// PushBack在列表l的后面插入一个新元素e，其值为v，并返回e。
func (l *List) PushBack(v interface{}) *Element {
	l.lazyInit()
	return l.insertValue(v, l.root.prev)
}

// InsertBefore在标记的前面插入一个值为v的新元素e，并返回e。如果标记不是l的元素，则列表不会被修改。标记不得为零。
func (l *List) InsertBefore(v interface{}, mark *Element) *Element {
	if mark.list != l {
		return nil
	}
	return l.insertValue(v, mark.prev)
}

// InsertAfter 在标记后面插入一个新元素e，值为v，然后返回e。如果标记不是l的元素，列表不会被修改。标记不得为零。
func (l *List) InsertAfter(v interface{}, mark *Element) *Element {
	if mark.list != l {
		return nil
	}
	return l.insertValue(v, mark)
}

// MoveToFront将元素e移动到列表l的前面。如果e不是l的元素，则列表不被修改。元素不能为nil。
func (l *List) MoveToFront(e *Element) {
	if e.list != l || l.root.next == e {
		return
	}
	l.move(e, &l.root)
}

// MoveToBack moves element e to the back of list l.
// If e is not an element of l, the list is not modified.
// The element must not be nil.
func (l *List) MoveToBack(e *Element) {
	if e.list != l || l.root.prev == e {
		return
	}
	// see comment in List.Remove about initialization of l
	l.move(e, l.root.prev)
}

// MoveToBack将元素e移动到列表l的后面，如果e不是l的元素，则列表不会被修改。元素不能为nil。
func (l *List) MoveBefore(e, mark *Element) {
	if e.list != l || e == mark || mark.list != l {
		return
	}
	l.move(e, mark.prev)
}

// MoveAfter 将元素e移动到标记后的新位置。如果e或mark不是l的元素，或e == mark，则不修改列表。元素和标记不能为空。
func (l *List) MoveAfter(e, mark *Element) {
	if e.list != l || e == mark || mark.list != l {
		return
	}
	l.move(e, mark)
}

// PushBackList在列表l的后面插入另一个列表的副本。列表l和其他列表可能是相同的。它们不能是零。
func (l *List) PushBackList(other *List) {
	l.lazyInit()
	for i, e := other.Len(), other.Front(); i > 0; i, e = i-1, e.Next() {
		l.insertValue(e.Value, l.root.prev)
	}
}

// PushFrontList在列表l的前面插入另一个列表的副本。列表l和其他列表可能是相同的。它们不能是零。
func (l *List) PushFrontList(other *List) {
	l.lazyInit()
	for i, e := other.Len(), other.Back(); i > 0; i, e = i-1, e.Prev() {
		l.insertValue(e.Value, &l.root)
	}
}
