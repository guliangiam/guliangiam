package route

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryInt_Present(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: "page=5"}}
	assert.Equal(t, 5, QueryInt(r, "page", 1))
}

func TestQueryInt_Missing(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: ""}}
	assert.Equal(t, 10, QueryInt(r, "page", 10))
}

func TestQueryInt_Invalid(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: "page=abc"}}
	assert.Equal(t, 1, QueryInt(r, "page", 1))
}

func TestQueryInt_Zero(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: "page=0"}}
	assert.Equal(t, 1, QueryInt(r, "page", 1))
}

func TestQueryInt_Negative(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: "page=-5"}}
	assert.Equal(t, 1, QueryInt(r, "page", 1))
}

func TestQueryOptionalBool_True(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: "active=true"}}
	result := QueryOptionalBool(r, "active")
	assert.NotNil(t, result)
	assert.True(t, *result)
}

func TestQueryOptionalBool_False(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: "active=false"}}
	result := QueryOptionalBool(r, "active")
	assert.NotNil(t, result)
	assert.False(t, *result)
}

func TestQueryOptionalBool_Missing(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: ""}}
	result := QueryOptionalBool(r, "active")
	assert.Nil(t, result)
}

func TestQueryOptionalBool_Invalid(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: "active=maybe"}}
	result := QueryOptionalBool(r, "active")
	assert.Nil(t, result)
}

func TestQueryOptionalString_Present(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: "name=test"}}
	result := QueryOptionalString(r, "name")
	assert.NotNil(t, result)
	assert.Equal(t, "test", *result)
}

func TestQueryOptionalString_Missing(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: ""}}
	result := QueryOptionalString(r, "name")
	assert.Nil(t, result)
}

func TestQueryOptionalString_Empty(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: "name="}}
	result := QueryOptionalString(r, "name")
	assert.Nil(t, result)
}
