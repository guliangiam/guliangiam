package idv

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func ekycOpts() InitiateOptions {
	return InitiateOptions{
		UserID:      "u-1",
		BizSeq:      "0123456789abcdef0123456789abcdef",
		MetaInfo:    `{"deviceType":"web"}`,
		RedirectURL: "https://iam.test/return",
		Country:     "MYS",
		Action:      ActionRealNameVerifyOverseas,
	}
}

func TestAliyunEKYC_InitiateRoutesToIntl(t *testing.T) {
	var form url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		form = r.PostForm
		_, _ = io.WriteString(w, `{"Code":"Success","Result":{"TransactionId":"txn-1","TransactionUrl":"https://intl.test/verify"}}`)
	}))
	defer server.Close()

	verifier := allowLoopbackAliyun(NewAliyunIDVVerifierWithOptions("ak", "sk", "cn-shanghai", "", server.URL, "", "eKYC_PRO", ""))
	result, err := verifier.InitiateVerification(context.Background(), ekycOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.VerificationID != "txn-1" || result.RedirectURL != "https://intl.test/verify" {
		t.Errorf("unexpected result: %+v", result)
	}
	if result.ProductCode != "eKYC_PRO" {
		t.Errorf("productCode = %q, want eKYC_PRO", result.ProductCode)
	}
	if form.Get("Action") != "InitializeV2" || form.Get("Version") != "2022-08-09" {
		t.Errorf("expected intl InitializeV2 2022-08-09, got Action=%q Version=%q", form.Get("Action"), form.Get("Version"))
	}
	if form.Get("MerchantBizId") != "0123456789abcdef0123456789abcdef" {
		t.Errorf("MerchantBizId = %q, want bizSeq", form.Get("MerchantBizId"))
	}
	if form.Get("DocType") != "MYS" {
		t.Errorf("DocType = %q, want country code MYS", form.Get("DocType"))
	}
}

func TestAliyunEKYC_InitiateRequiresBizSeqAndMetaInfo(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = io.WriteString(w, `{"Code":"Success"}`)
	}))
	defer server.Close()
	verifier := allowLoopbackAliyun(NewAliyunIDVVerifierWithOptions("ak", "sk", "cn-shanghai", "", server.URL, "", "eKYC_PRO", ""))

	opts := ekycOpts()
	opts.BizSeq = ""
	if _, err := verifier.InitiateVerification(context.Background(), opts); err == nil {
		t.Error("missing bizSeq must fail closed")
	}
	opts = ekycOpts()
	opts.MetaInfo = ""
	if _, err := verifier.InitiateVerification(context.Background(), opts); err == nil {
		t.Error("missing metaInfo must fail closed")
	}
	if calls != 0 {
		t.Errorf("validation must precede the upstream call, got %d calls", calls)
	}
}

func TestAliyunEKYC_CheckResultRequiresOrder(t *testing.T) {
	verifier := allowLoopbackAliyun(NewAliyunIDVVerifierWithOptions("ak", "sk", "cn-shanghai", "", "https://cloudauth-intl.test", "", "eKYC_PRO", ""))
	// Plain reference query cannot serve an intl product.
	_, err := verifier.GetVerificationResult(context.Background(), "txn-1")
	if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeInvalidArgument {
		t.Fatalf("intl plain query must be rejected, got %v", err)
	}
	// Order-aware query without the merchant sequence is also rejected.
	if _, err := verifier.GetVerificationResultWithOrder(context.Background(), "txn-1", ""); err == nil {
		t.Error("missing merchantBizId must fail closed")
	}
}

func TestAliyunEKYC_CheckResultMapsPassedAndScore(t *testing.T) {
	var form url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		form = r.PostForm
		_, _ = io.WriteString(w, `{"Code":"Success","Result":{"Passed":"Y","SubCode":"200","ExtFaceInfo":"{\"faceComparisonScore\":99.5,\"facePassed\":\"Y\"}"}}`)
	}))
	defer server.Close()

	verifier := allowLoopbackAliyun(NewAliyunIDVVerifierWithOptions("ak", "sk", "cn-shanghai", "", server.URL, "", "eKYC_PRO", ""))
	result, err := verifier.GetVerificationResultWithOrder(context.Background(), "txn-1", "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusApproved {
		t.Errorf("status = %s, want approved", result.Status)
	}
	if result.GatewayCode != "200" {
		t.Errorf("gatewayCode = %q, want 200", result.GatewayCode)
	}
	if result.Confidence != 99.5 {
		t.Errorf("confidence = %v, want 99.5", result.Confidence)
	}
	if form.Get("Action") != "CheckResult" || form.Get("MerchantBizId") == "" || form.Get("TransactionId") != "txn-1" {
		t.Errorf("expected CheckResult with order, got %v", form)
	}
}

func TestAliyunEKYC_CheckResultRejectedReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"Code":"Success","Result":{"Passed":"N","SubCode":"204"}}`)
	}))
	defer server.Close()
	verifier := allowLoopbackAliyun(NewAliyunIDVVerifierWithOptions("ak", "sk", "cn-shanghai", "", server.URL, "", "eKYC_PRO", ""))
	result, err := verifier.GetVerificationResultWithOrder(context.Background(), "txn-1", "biz-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusRejected || result.Reason == "" {
		t.Errorf("expected rejected with a reason, got %+v", result)
	}
}

func TestAliyunLoginVerifyRoutesToPVFV(t *testing.T) {
	var form url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		form = r.PostForm
		_, _ = io.WriteString(w, `{"Code":"200","ResultObject":"{\"certifyId\":\"cert-1\",\"certifyUrl\":\"https://aliyun.test/certify\"}"}`)
	}))
	defer server.Close()

	verifier := allowLoopbackAliyun(NewAliyunIDVVerifierWithOptions("ak", "sk", "cn-shanghai", "", server.URL, "scene-1", "", ""))
	result, err := verifier.InitiateVerification(context.Background(), InitiateOptions{
		Action: ActionLoginVerify, UserID: "u-1",
		BizSeq: "0123456789abcdef0123456789abcdef", MetaInfo: `{"deviceType":"h5"}`,
		RedirectURL: "https://iam.test/return", TargetFaceURL: "https://cdn.test/face.jpg",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ProductCode != "PV_FV" {
		t.Errorf("productCode = %q, want PV_FV", result.ProductCode)
	}
	if form.Get("ProductCode") != "PV_FV" || form.Get("FaceContrastPictureUrl") != "https://cdn.test/face.jpg" {
		t.Errorf("expected PV_FV with the enrolled face reference, got %v", form)
	}
	if form.Get("CertifyUrlType") != "H5" {
		t.Errorf("CertifyUrlType = %q, want H5", form.Get("CertifyUrlType"))
	}
}

func TestAliyunLoginVerifyRequiresFaceAndUser(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = io.WriteString(w, `{"Code":"200"}`)
	}))
	defer server.Close()
	verifier := allowLoopbackAliyun(NewAliyunIDVVerifierWithOptions("ak", "sk", "cn-shanghai", "", server.URL, "scene-1", "", ""))

	base := InitiateOptions{
		Action: ActionLoginVerify, BizSeq: "0123456789abcdef0123456789abcdef",
		MetaInfo: `{"deviceType":"web"}`, RedirectURL: "https://iam.test/return",
	}
	if _, err := verifier.InitiateVerification(context.Background(), base); err == nil {
		t.Error("login verification without an enrolled face must fail closed")
	}
	opts := base
	opts.TargetFaceURL = "https://cdn.test/face.jpg"
	if _, err := verifier.InitiateVerification(context.Background(), opts); err == nil {
		t.Error("login verification without userId must fail closed")
	}
	if calls != 0 {
		t.Errorf("validation must precede the upstream call, got %d calls", calls)
	}
}

func TestAliyunDomesticResultExtractsVerifyScore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"Code":"200","ResultObject":"{\"passed\":\"T\",\"subCode\":\"200\",\"materialInfo\":\"{\\\"facialPictureFront\\\":{\\\"verifyScore\\\":88.8}}\"}"}`)
	}))
	defer server.Close()
	verifier := allowLoopbackAliyun(NewAliyunIDVVerifierWithOptions("ak", "sk", "cn-shanghai", "", server.URL, "scene-1", "", ""))
	result, err := verifier.GetVerificationResult(context.Background(), "cert-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusApproved || result.Confidence != 88.8 {
		t.Errorf("expected approved with verifyScore 88.8, got %+v", result)
	}
	if result.GatewayCode != "200" {
		t.Errorf("gatewayCode = %q, want 200", result.GatewayCode)
	}
}

func TestIsAliyunIntlProduct(t *testing.T) {
	intl := []string{"eKYC_PRO", "eKYC", "eKYC_NDI", "FACE_IDU", "FACE_VERIFY", "FACE_LIVENESS_PRO", "ID_OCR_PRO"}
	for _, code := range intl {
		if !isAliyunIntlProduct(code) {
			t.Errorf("%q must be classified as intl", code)
		}
	}
	domestic := []string{"ID_PRO", "ID_PLUS", "PV_FV", "LR_FR", ""}
	for _, code := range domestic {
		if isAliyunIntlProduct(code) {
			t.Errorf("%q must be classified as domestic", code)
		}
	}
}
