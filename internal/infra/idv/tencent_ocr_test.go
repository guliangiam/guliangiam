package idv

import (
	"context"
	"encoding/base64"
	"encoding/json"
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

func allowLoopbackTencentOCR(recognizer *TencentOCRRecognizer) *TencentOCRRecognizer {
	recognizer.client = ssrf.SafeHTTPClientWithAllowlist(time.Second, []string{"127.0.0.1/32"})
	return recognizer
}

func newTestTencentOCRRecognizer(endpoint string) *TencentOCRRecognizer {
	return allowLoopbackTencentOCR(NewTencentOCRRecognizer(OCRConfig{
		Provider:  tencentOCRProviderName,
		AccessKey: "AKIDtest",
		SecretKey: "sktest",
		Endpoint:  endpoint,
	}))
}

func TestTencentOCR_RecognizeIDCard(t *testing.T) {
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-TC-Action"); got != "IDCardOCR" {
			t.Errorf("X-TC-Action = %q", got)
		}
		if got := r.Header.Get("X-TC-Version"); got != "2018-11-19" {
			t.Errorf("X-TC-Version = %q", got)
		}
		if got := r.Header.Get("X-TC-Region"); got != tencentOCRDefaultRegion {
			t.Errorf("X-TC-Region = %q, want %q", got, tencentOCRDefaultRegion)
		}
		gotBody, _ = io.ReadAll(r.Body)
		verifyTencentTC3(t, r, gotBody, "AKIDtest", "sktest", "ocr")
		_, _ = io.WriteString(w, `{"Response":{"RequestId":"req-1","Name":"张三","Sex":"男","Nation":"汉","Birth":"1990年1月1日","Address":"上海市浦东新区","IdNum":"110101199001011234","Authority":"上海市公安局浦东分局","ValidDate":"20200101-20400101","AdvancedInfo":"{\"Quality\":92}"}}`)
	}))
	defer server.Close()

	recognizer := newTestTencentOCRRecognizer(server.URL)
	fields, err := recognizer.RecognizeDocument(context.Background(), domainidv.DocumentIDCard, DocumentImage{
		Data:      []byte("fake-image"),
		MediaType: "image/png",
		Side:      "front",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body := string(gotBody)
	wantEncoded := base64.StdEncoding.EncodeToString([]byte("fake-image"))
	if !strings.Contains(body, `"ImageBase64":"`+wantEncoded+`"`) {
		t.Errorf("image bytes were not sent base64-encoded: %s", body)
	}
	if !strings.Contains(body, `"CardSide":"FRONT"`) {
		t.Errorf("front side hint not forwarded: %s", body)
	}
	if fields.FullName != "张三" || fields.IDNumber != "110101199001011234" {
		t.Errorf("unexpected fields: %+v", fields)
	}
	if fields.Gender != "male" || fields.Birthday != "1990-01-01" {
		t.Errorf("unexpected personal fields: %+v", fields)
	}
	if fields.IssuingAuthority != "上海市公安局浦东分局" {
		t.Errorf("unexpected authority: %q", fields.IssuingAuthority)
	}
	if fields.ValidFrom != "2020-01-01" || fields.ValidTo != "2040-01-01" {
		t.Errorf("unexpected validity: %q - %q", fields.ValidFrom, fields.ValidTo)
	}
	if fields.Confidence < 0.91 || fields.Confidence > 0.93 {
		t.Errorf("confidence must be normalized to 0-1, got %v", fields.Confidence)
	}
	if fields.IDType != string(domainidv.DocumentIDCard) {
		t.Errorf("idType = %q", fields.IDType)
	}
}

func TestTencentOCR_PassportUsesMLIDAction(t *testing.T) {
	var gotAction string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAction = r.Header.Get("X-TC-Action")
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, `{"Response":{"RequestId":"req-2","ID":"E12345678","Name":"ZHANG SAN","DateOfBirth":"19900101","Sex":"M","DateOfExpiration":"20300101","IssuingCountry":"CHN","Nationality":"CHN"}}`)
	}))
	defer server.Close()

	recognizer := newTestTencentOCRRecognizer(server.URL)
	fields, err := recognizer.RecognizeDocument(context.Background(), domainidv.DocumentPassport, DocumentImage{Data: []byte("x")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAction != "MLIDPassportOCR" {
		t.Errorf("action = %q, want MLIDPassportOCR", gotAction)
	}
	if strings.Contains(string(gotBody), "CardSide") {
		t.Errorf("passports must not send CardSide: %s", string(gotBody))
	}
	if fields.IDNumber != "E12345678" || fields.FullName != "ZHANG SAN" || fields.Nationality != "CHN" {
		t.Errorf("unexpected fields: %+v", fields)
	}
	if fields.Gender != "male" || fields.Birthday != "1990-01-01" || fields.ValidTo != "2030-01-01" {
		t.Errorf("unexpected passport fields: %+v", fields)
	}
}

func TestTencentOCR_DriverLicense(t *testing.T) {
	var gotAction string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAction = r.Header.Get("X-TC-Action")
		_, _ = io.WriteString(w, `{"Response":{"RequestId":"req-3","Name":"李四","Sex":"女","Nationality":"中国","Address":"北京市朝阳区","DateOfBirth":"19880505","Class":"C1","StartDate":"20210601","EndDate":"20270601","CardCode":"110101198805051234","IssuingAuthority":"北京市公安局交通管理局"}}`)
	}))
	defer server.Close()

	recognizer := newTestTencentOCRRecognizer(server.URL)
	fields, err := recognizer.RecognizeDocument(context.Background(), domainidv.DocumentDriverLicense, DocumentImage{Data: []byte("x"), Side: "back"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAction != "DriverLicenseOCR" {
		t.Errorf("action = %q, want DriverLicenseOCR", gotAction)
	}
	if fields.FullName != "李四" || fields.IDNumber != "110101198805051234" || fields.ValidFrom != "2021-06-01" || fields.ValidTo != "2027-06-01" {
		t.Errorf("unexpected fields: %+v", fields)
	}
}

func TestTencentOCR_BusinessErrorMapping(t *testing.T) {
	cases := map[string]apperrors.Code{
		`{"Response":{"Error":{"Code":"InvalidParameter","Message":"图片格式不支持"},"RequestId":"r"}}`:           apperrors.CodeInvalidArgument,
		`{"Response":{"Error":{"Code":"AuthFailure.SecretIdNotFound","Message":"密钥不存在"},"RequestId":"r"}}`: apperrors.CodeUnauthorized,
		`{"Response":{"Error":{"Code":"FailedOperation.ImageNoText","Message":"未检测到文字"},"RequestId":"r"}}`: apperrors.CodeUnavailable,
	}
	for payload, want := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, payload)
		}))
		recognizer := newTestTencentOCRRecognizer(server.URL)
		_, err := recognizer.RecognizeDocument(context.Background(), domainidv.DocumentIDCard, DocumentImage{Data: []byte("x")})
		server.Close()
		appErr, ok := err.(*apperrors.Error)
		if !ok || appErr.Code != want {
			t.Fatalf("payload %s: expected code %v, got %v", payload, want, err)
		}
	}
}

func TestTencentOCR_UnsupportedDocumentType(t *testing.T) {
	recognizer := newTestTencentOCRRecognizer("http://127.0.0.1:1")
	_, err := recognizer.RecognizeDocument(context.Background(), domainidv.DocumentType("military_id"), DocumentImage{Data: []byte("x")})
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeInvalidArgument {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestTencentOCR_RejectsMissingCredentials(t *testing.T) {
	recognizer := allowLoopbackTencentOCR(NewTencentOCRRecognizer(OCRConfig{Provider: tencentOCRProviderName}))
	_, err := recognizer.RecognizeDocument(context.Background(), domainidv.DocumentIDCard, DocumentImage{Data: []byte("x")})
	if err == nil {
		t.Fatal("expected an error without credentials")
	}
}

func TestTencentOCR_NoReadableFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"Response":{"RequestId":"req-4","Name":"","IdNum":""}}`)
	}))
	defer server.Close()

	recognizer := newTestTencentOCRRecognizer(server.URL)
	_, err := recognizer.RecognizeDocument(context.Background(), domainidv.DocumentIDCard, DocumentImage{Data: []byte("x")})
	if err == nil || !strings.Contains(err.Error(), "no readable identity fields") {
		t.Fatalf("expected readable-field error, got %v", err)
	}
}

func TestTencentOCR_RegionOverride(t *testing.T) {
	var gotRegion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRegion = r.Header.Get("X-TC-Region")
		_, _ = io.WriteString(w, `{"Response":{"RequestId":"req-5","Name":"王五","IdNum":"110101199001011234"}}`)
	}))
	defer server.Close()

	recognizer := allowLoopbackTencentOCR(NewTencentOCRRecognizer(OCRConfig{
		Provider:  tencentOCRProviderName,
		AccessKey: "AKIDtest",
		SecretKey: "sktest",
		Region:    "ap-shanghai",
		Endpoint:  server.URL,
	}))
	if _, err := recognizer.RecognizeDocument(context.Background(), domainidv.DocumentIDCard, DocumentImage{Data: []byte("x")}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotRegion != "ap-shanghai" {
		t.Errorf("X-TC-Region = %q, want ap-shanghai", gotRegion)
	}
}

// TestTencentOCR_DateAndSexNormalization pins the DATE-column and gender
// normalisation across the provider's date shapes (yyyyMMdd, yyyy年M月d日,
// yyyy.MM.dd) and the 长期/长期有效 open-ended validity marker.
func TestTencentOCR_DateAndSexNormalization(t *testing.T) {
	t.Run("date_formats", func(t *testing.T) {
		cases := []struct{ in, want string }{
			{"19900101", "1990-01-01"},
			{"1990年1月1日", "1990-01-01"},
			{"1990.01.01", "1990-01-01"},
			{"1990-01-01", "1990-01-01"},
			{"2020年12月31日", "2020-12-31"},
			{"2020.12.31", "2020-12-31"},
			{"garbage", ""},
			{"", ""},
		}
		for _, c := range cases {
			if got := normalizeTencentOCRDate(c.in); got != c.want {
				t.Errorf("normalizeTencentOCRDate(%q) = %q, want %q", c.in, got, c.want)
			}
		}
	})

	t.Run("long_term_validity_maps_to_empty", func(t *testing.T) {
		for _, marker := range []string{"长期", "长期有效"} {
			fields, err := parseTencentOCRResponse(domainidv.DocumentIDCard, json.RawMessage(
				`{"Name":"张三","IdNum":"110101199001011234","Birth":"1990年1月1日","Sex":"男","ValidDate":"20200101-`+marker+`"}`))
			if err != nil {
				t.Fatalf("marker %s: %v", marker, err)
			}
			if fields.ValidFrom != "2020-01-01" {
				t.Errorf("ValidFrom = %q, want 2020-01-01", fields.ValidFrom)
			}
			if fields.ValidTo != "" {
				t.Errorf("ValidTo for %s = %q, want empty", marker, fields.ValidTo)
			}
		}
	})

	t.Run("sex_values", func(t *testing.T) {
		cases := []struct{ in, want string }{
			{"男", "male"},
			{"女", "female"},
			{"M", "male"},
			{"F", "female"},
			{"m", "male"},
			{"f", "female"},
			{"unknown", ""},
			{"", ""},
		}
		for _, c := range cases {
			if got := normalizeTencentOCRSex(c.in); got != c.want {
				t.Errorf("normalizeTencentOCRSex(%q) = %q, want %q", c.in, got, c.want)
			}
		}
	})

	t.Run("driver_license_dates_normalized", func(t *testing.T) {
		fields, err := parseTencentOCRResponse(domainidv.DocumentDriverLicense, json.RawMessage(
			`{"Name":"李四","Sex":"女","DateOfBirth":"1988.5.5","StartDate":"20210601","EndDate":"长期"}`))
		if err != nil {
			t.Fatal(err)
		}
		if fields.Birthday != "1988-05-05" || fields.ValidFrom != "2021-06-01" || fields.ValidTo != "" || fields.Gender != "female" {
			t.Errorf("unexpected normalized fields: %+v", fields)
		}
	})
}
