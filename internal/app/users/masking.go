package users

import (
	"fmt"
	"regexp"
	"strings"
)

var rePhone = regexp.MustCompile(`(\d{3})\d*(\d{4})`)

func getMaskedEmail(email string) string {
	if email == "" {
		return ""
	}
	if !strings.Contains(email, "@") {
		return maskString(email)
	}
	tokens := strings.Split(email, "@")
	username := maskString(tokens[0])
	domain := tokens[1]
	domainTokens := strings.Split(domain, ".")
	if len(domainTokens) >= 2 {
		domainTokens[len(domainTokens)-2] = maskString(domainTokens[len(domainTokens)-2])
	}
	return fmt.Sprintf("%s@%s", username, strings.Join(domainTokens, "."))
}

func getMaskedPhone(phone string) string {
	return rePhone.ReplaceAllString(phone, "$1****$2")
}

func maskString(str string) string {
	// BE-A-17: 按 rune 处理，避免多字节 UTF-8 字符被截半产生乱码。
	runes := []rune(str)
	if len(runes) <= 2 {
		return str
	}
	return fmt.Sprintf("%c%s%c", runes[0], strings.Repeat("*", len(runes)-2), runes[len(runes)-1])
}
