package notificationproviders

import enumset "github.com/guliangiam/guliangiam/internal/platform/enumset"

// ProviderTypes is the closed set of notification provider types a provider
// record may name.
//
// The values are the keys the notification factory is registered under
// (`internal/bootstrap/runtime.go`). The console previously offered
// `DingTalk`/`WeCom`/`Lark`/`Telegram`/`Discord`/`Slack`/`Bark`/`CustomHTTP`,
// none of which matched a registered key, so a provider created in the console
// could never deliver anything.
//
// `email`/`sms` are present but register only when a sender is available, for
// the same pre-configuration reason as the captcha types.
//
// `customhttp`, `googlechat`, `cucloud`, `ntfy`, `serverchan`, `pagerduty`,
// `opsgenie` and `gotify` take their endpoint/credentials from the provider
// record, so the factory builds them per delivery.
var ProviderTypes = enumset.NewSet(
	"notificationProviderType",
	"email",
	"sms",
	"dingtalk",
	"lark",
	"wecom",
	// wecom_app 是自建应用消息（按 WeCom userid 单发），与上面的群机器人
	// webhook 型 "wecom" 不是同一个渠道；wechat_official 是公众号模板消息。
	"wecom_app",
	"wechat_official",
	"slack",
	"discord",
	"rocketchat",
	"teams",
	"bark",
	"telegram",
	"pushover",
	"line",
	"matrix",
	"pushbullet",
	"viber",
	"webpush",
	"twitter",
	"reddit",
	"customhttp",
	"googlechat",
	"cucloud",
	"ntfy",
	"serverchan",
	"pagerduty",
	"opsgenie",
	"gotify",
)
