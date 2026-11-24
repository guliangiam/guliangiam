package messaging

// 第三方邮件/短信网关请求体里反复出现的字段名。同一字段名散落成多个字面量时，
// 改名会漏改其中一处，而漏改只在真实发送时才会暴露；集中成常量后由编译器保证一致。
const (
	fieldFrom    = "from"
	fieldSubject = "subject"
	fieldText    = "text"
	fieldContent = "content"
	fieldCode    = "code"
)
