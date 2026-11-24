package notification

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBarkNotifier(t *testing.T) {
	n := NewBarkNotifier()
	assert.NotNil(t, n)
}

func TestNewCustomHTTPNotifier(t *testing.T) {
	n := NewCustomHTTPNotifier("https://example.com/webhook", map[string]string{"X-Auth": "token"})
	assert.NotNil(t, n)
}

func TestNewDingTalkNotifier(t *testing.T) {
	n := NewDingTalkNotifier()
	assert.NotNil(t, n)
}

func TestNewDiscordNotifier(t *testing.T) {
	n := NewDiscordNotifier()
	assert.NotNil(t, n)
}

func TestNewEmailNotifier(t *testing.T) {
	n := NewEmailNotifier(nil, "from@example.com", "Subject")
	assert.NotNil(t, n)
}

func TestNewGoogleChatNotifier(t *testing.T) {
	n := NewGoogleChatNotifier("https://chat.googleapis.com/v1/spaces/xxx/messages")
	assert.NotNil(t, n)
}

func TestNewCUCloudNotifier(t *testing.T) {
	n := NewCUCloudNotifier("appID", "apiKey")
	assert.NotNil(t, n)
}

func TestNewLarkNotifier(t *testing.T) {
	n := NewLarkNotifier()
	assert.NotNil(t, n)
}

func TestNewLineNotifier(t *testing.T) {
	n := NewLineNotifier("accessToken")
	assert.NotNil(t, n)
}

func TestNewMatrixNotifier(t *testing.T) {
	n := NewMatrixNotifier("https://matrix.example.com", "accessToken")
	assert.NotNil(t, n)
}

func TestNewPushbulletNotifier(t *testing.T) {
	n := NewPushbulletNotifier("accessToken")
	assert.NotNil(t, n)
}

func TestNewPushoverNotifier(t *testing.T) {
	n := NewPushoverNotifier("appToken")
	assert.NotNil(t, n)
}

func TestNewRedditNotifier(t *testing.T) {
	n := NewRedditNotifier()
	assert.NotNil(t, n)
}

func TestNewRedditNotifierWithConfig(t *testing.T) {
	n := NewRedditNotifierWithConfig("accessToken", "clientID", "clientSecret", "username")
	assert.NotNil(t, n)
}

func TestNewRocketChatNotifier(t *testing.T) {
	n := NewRocketChatNotifier()
	assert.NotNil(t, n)
}

func TestNewSlackNotifier(t *testing.T) {
	n := NewSlackNotifier()
	assert.NotNil(t, n)
}

func TestNewSmsNotifier(t *testing.T) {
	n := NewSmsNotifier(nil)
	assert.NotNil(t, n)
}

func TestNewTeamsNotifier(t *testing.T) {
	n := NewTeamsNotifier()
	assert.NotNil(t, n)
}

func TestNewTelegramNotifier(t *testing.T) {
	n := NewTelegramNotifier("botToken")
	assert.NotNil(t, n)
}

func TestNewTwitterNotifier(t *testing.T) {
	n := NewTwitterNotifier()
	assert.NotNil(t, n)
}

func TestNewTwitterNotifierWithConfig(t *testing.T) {
	n := NewTwitterNotifierWithConfig("token", "apiKey", "apiSecret")
	assert.NotNil(t, n)
}

func TestNewViberNotifier(t *testing.T) {
	n := NewViberNotifier()
	assert.NotNil(t, n)
}

func TestNewViberNotifierWithConfig(t *testing.T) {
	n := NewViberNotifierWithConfig("authToken", "name")
	assert.NotNil(t, n)
}

func TestNewWebPushNotifier(t *testing.T) {
	n := NewWebPushNotifier()
	assert.NotNil(t, n)
}

func TestNewWebPushNotifierWithConfig(t *testing.T) {
	n := NewWebPushNotifierWithConfig("publicKey", "privateKey", "email")
	assert.NotNil(t, n)
}

func TestNewWeComNotifier(t *testing.T) {
	n := NewWeComNotifier()
	assert.NotNil(t, n)
}
