package strings

// Compare返回一个整数，该整数按词法比较两个字符串。如果a==b，结果为0，如果a < b，结果为-1，如果a > b，结果为+1。使用内置的字符串比较运算符==、<、>等通常更清晰，而且总是更快。
func Compare(a, b string) int {
	// Note:这个函数不调用运行时的cmpstring函数，因为我们不想为使用strings.Compare提供任何性能证明。基本上，没有人应该使用字符串。正如上面的注释所说，这里只是为了与包字节对称。
	// 如果性能很重要，那么应该改变编译器来识别这种模式，这样所有进行三向比较的代码，而不仅仅是使用strings.Compare的代码，都可以受益。
	if a == b {
		return 0
	}
	if a < b {
		return -1
	}
	return +1
}
