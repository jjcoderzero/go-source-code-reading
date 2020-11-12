package time

import "errors"

// 一个Ticker持有一个通道，它每隔一段时间就发送一个时钟的“滴答声”。
type Ticker struct {
	C <-chan Time // 传输滴答声的通道。
	r runtimeTimer
}

// NewTicker返回一个包含通道的Ticker，该通道将发送带有duration参数指定的时间段的时间。它调整间隔或滴答，以弥补慢Ticker。持续时间d必须大于零;否则，NewTicker将会恐慌。停止Ticker以释放相关的资源。
func NewTicker(d Duration) *Ticker {
	if d <= 0 {
		panic(errors.New("non-positive interval for NewTicker"))
	}
	// 给通道一个1元素的时间缓冲。如果客户在阅读时落后了，我们就在地板上扔滴答声，直到客户赶上来。
	c := make(chan Time, 1)
	t := &Ticker{
		C: c,
		r: runtimeTimer{
			when:   when(d),
			period: int64(d),
			f:      sendTime,
			arg:    c,
		},
	}
	startTimer(&t.r)
	return t
}

// Stop停止a ticker. 停止后，将不再发送节拍。停止不关闭通道，以防止同时从通道读取goroutine看到一个错误的“滴答”。
func (t *Ticker) Stop() {
	stopTimer(&t.r)
}

// Reset停止报价器并将其周期重置为指定的持续时间。下一个滴答将在新时期结束后到达。
func (t *Ticker) Reset(d Duration) {
	if t.r.f == nil {
		panic("time: Reset called on uninitialized Ticker")
	}
	modTimer(&t.r, when(d), int64(d), t.r.f, t.r.arg, t.r.seq)
}

// Tick是一个方便的包装NewTicker提供访问滴答通道。滴答是有用的客户端，没有必要关闭的报价机，请注意，没有办法关闭它，底层报价机无法恢复的垃圾收集器;它“泄漏”。与NewTicker不同，Tick在d <= 0时返回nil。
func Tick(d Duration) <-chan Time {
	if d <= 0 {
		return nil
	}
	return NewTicker(d).C
}
