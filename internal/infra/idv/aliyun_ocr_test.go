package idv

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domainidv "github.com/guliangiam/guliangiam/internal/domain/idv"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	ssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

func allowLoopbackOCR(recognizer *AliyunOCRRecognizer) *AliyunOCRRecognizer {
	recognizer.client = ssrf.SafeHTTPClientWithAllowlist(time.Second, []string{"127.0.0.1/32"})
	return recognizer
}

func newTestOCRRecognizer(endpoint string) *AliyunOCRRecognizer {
	return allowLoopbackOCR(NewAliyunOCRRecognizer(OCRConfig{
		Provider:  aliyunOCRProviderName,
		AccessKey: "ak",
		SecretKey: "sk",
		Endpoint:  endpoint,
	}))
}

func TestAliyunOCR_RecognizeIDCard(t *testing.T) {
	var gotAction string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAction = r.URL.Query().Get("Action")
		if r.URL.Query().Get("Version") != aliyunOCRAPIVersion {
			t.Errorf("unexpected version %q", r.URL.Query().Get("Version"))
		}
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, `{"RequestId":"req-1","Data":"{\"data\":{\"face\":{\"name\":\"张三\",\"sex\":\"男\",\"birthDate\":\"1990年1月1日\",\"address\":\"上海市浦东新区\",\"idNumber\":\"110101199001011234\"},\"back\":{\"issueAuthority\":\"上海市公安局\",\"validPeriod\":\"2020.01.01-2040.01.01\"}},\"warning\":{\"qualityScore\":92}}"}`)
	}))
	defer server.Close()

	recognizer := newTestOCRRecognizer(server.URL)
	fields, err := recognizer.RecognizeDocument(context.Background(), domainidv.DocumentIDCard, DocumentImage{
		Data:      []byte("fake-image"),
		MediaType: "image/png",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAction != "RecognizeIdcard" {
		t.Errorf("unexpected action %q", gotAction)
	}
	if string(gotBody) != "fake-image" {
		t.Errorf("image bytes were not sent as the raw body: %q", string(gotBody))
	}
	if fields.FullName != "张三" || fields.IDNumber != "110101199001011234" {
		t.Errorf("unexpected fields: %+v", fields)
	}
	if fields.Gender != "男" || fields.Birthday != "1990年1月1日" {
		t.Errorf("unexpected personal fields: %+v", fields)
	}
	if fields.IssuingAuthority != "上海市公安局" {
		t.Errorf("unexpected authority: %q", fields.IssuingAuthority)
	}
	if fields.ValidFrom != "2020.01.01" || fields.ValidTo != "2040.01.01" {
		t.Errorf("unexpected validity: %q - %q", fields.ValidFrom, fields.ValidTo)
	}
	if fields.Confidence <= 0 || fields.Confidence > 1 {
		t.Errorf("confidence must be normalized to 0-1, got %v", fields.Confidence)
	}
}

func TestAliyunOCR_ProviderErrorIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"Code":"Throttling.User","Message":"too many requests"}`)
	}))
	defer server.Close()

	recognizer := newTestOCRRecognizer(server.URL)
	_, err := recognizer.RecognizeDocument(context.Background(), domainidv.DocumentPassport, DocumentImage{Data: []byte("x")})
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeUnavailable {
		t.Fatalf("expected unavailable, got %v", err)
	}
}

// Aliyun reports an unknown AccessKey as HTTP 404; the body code must win over
// the status so the operator sees "credential rejected" instead of "not found".
func TestAliyunOCR_AccessKeyNotFoundKeepsBodyCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"Code":"InvalidAccessKeyId.NotFound","Message":"Specified access key is not found."}`)
	}))
	defer server.Close()

	recognizer := newTestOCRRecognizer(server.URL)
	_, err := recognizer.RecognizeDocument(context.Background(), domainidv.DocumentIDCard, DocumentImage{Data: []byte("x")})
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
	if !strings.Contains(appErr.Message, "aliyun_ocr: credential rejected") {
		t.Fatalf("expected provider-prefixed message, got %q", appErr.Message)
	}
}

func TestAliyunOCR_UnsupportedDocumentType(t *testing.T) {
	recognizer := newTestOCRRecognizer("http://127.0.0.1:1")
	_, err := recognizer.RecognizeDocument(context.Background(), domainidv.DocumentType("military_id"), DocumentImage{Data: []byte("x")})
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeInvalidArgument {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestAliyunOCR_NoReadableFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"RequestId":"req-2","Data":"{\"data\":{}}"}`)
	}))
	defer server.Close()

	recognizer := newTestOCRRecognizer(server.URL)
	_, err := recognizer.RecognizeDocument(context.Background(), domainidv.DocumentIDCard, DocumentImage{Data: []byte("x")})
	if err == nil || !strings.Contains(err.Error(), "no readable identity fields") {
		t.Fatalf("expected readable-field error, got %v", err)
	}
}

// OCR 与 CloudAuth 共用分类器：未知码坍缩为 503 时，Meta 必须携带真实
// 阿里云码，否则前端只剩「服务暂时不可用」，运维无法定位开通/授权问题。
func TestAliyunOCR_UnknownCodeCarriesProviderCodeMeta(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"Code":"InvalidProduct.NotActivated","Message":"ocr product not activated"}`)
	}))
	defer server.Close()

	recognizer := newTestOCRRecognizer(server.URL)
	_, err := recognizer.RecognizeDocument(context.Background(), domainidv.DocumentIDCard, DocumentImage{Data: []byte("x")})
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeUnavailable {
		t.Fatalf("expected unavailable, got %v", err)
	}
	if got := appErr.Meta["providerCode"]; got != "InvalidProduct.NotActivated" {
		t.Fatalf("meta[providerCode] = %v, want InvalidProduct.NotActivated", got)
	}
	if got := appErr.Meta["provider"]; got != aliyunOCRProviderName {
		t.Fatalf("meta[provider] = %v, want %q", got, aliyunOCRProviderName)
	}
}
