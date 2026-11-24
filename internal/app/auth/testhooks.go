package auth

import "context"

type PushProviderConfigForTest = pushProviderConfig

type SendPushAuthenticationFuncForTest func(ctx context.Context, config PushProviderConfigForTest, receiver string, title string, message string) error

func SendPushAuthenticationForTest() SendPushAuthenticationFuncForTest {
	return sendPushAuthentication
}

func SetSendPushAuthenticationForTest(fn SendPushAuthenticationFuncForTest) {
	if fn == nil {
		return
	}
	sendPushAuthentication = fn
}
