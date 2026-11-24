package entries

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	entrydomain "github.com/guliangiam/guliangiam/internal/domain/entries"
	igadomain "github.com/guliangiam/guliangiam/internal/domain/iga"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/redact"
)

// mockRecordRepo records the audit rows written while reading a transcript, so
// the tests can assert that reading is itself an audited event.
type mockRecordRepo struct {
	created []*recordsdomain.Record
}

func (r *mockRecordRepo) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

func (r *mockRecordRepo) List(context.Context, recordsdomain.ListQuery) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *mockRecordRepo) Search(context.Context, recordsdomain.SearchCriteria) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}
func (r *mockRecordRepo) LoadLastChainEntry(context.Context, string) (seq int64, hash []byte, err error) {
	return 0, nil, nil
}
func (r *mockRecordRepo) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}
func (r *mockRecordRepo) InsertAnchor(context.Context, *recordsdomain.ChainAnchor) error {
	return nil
}
func (r *mockRecordRepo) ListChainEntries(context.Context, string) ([]recordsdomain.ChainEntry, error) {
	return nil, nil
}
func (r *mockRecordRepo) IGA() igadomain.Repository { return nil }

// newTranscriptSvc builds a service whose storage holds exactly one entry, so
// every test can reason about a single stream.
func newTranscriptSvc(entry *entrydomain.Entry) (Service, *mockRecordRepo) {
	records := &mockRecordRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: fixedTestTime()},
		UUID:  fakeUUIDGenerator{fixedID: "test-id"},
		UOW:   testUnitOfWork{entryRepo: &mockEntryRepo{entries: []*entrydomain.Entry{entry}}, recordRepo: records},
	}
	return svc, records
}

func TestListEntryTranscript_ReturnsMessagesInAppendOrder(t *testing.T) {
	entry := &entrydomain.Entry{
		ID: "entry-1", OrganizationID: "org-1", Type: "trace",
		CreatedAt: fixedTestTime(),
		Message: `{"resourceSpans":[{"scopeSpans":[{"scope":{"name":"web"},
			"spans":[
				{"name":"first","startTimeUnixNano":"1700000000000000001"},
				{"name":"second","startTimeUnixNano":"1700000000000000002"},
				{"name":"third","startTimeUnixNano":"1700000000000000003"}
			]}]}]}`,
	}
	svc, _ := newTranscriptSvc(entry)

	page, err := svc.ListEntryTranscript(context.Background(), TranscriptQuery{
		EntryID: "entry-1", OrganizationID: "org-1",
	})
	if err != nil {
		t.Fatalf("ListEntryTranscript: %v", err)
	}

	if len(page.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(page.Messages))
	}
	for index, message := range page.Messages {
		if message.Index != index {
			t.Errorf("message %d has index %d; the index must be its append position", index, message.Index)
		}
		if message.Kind != transcriptKindSpan {
			t.Errorf("message %d has kind %q", index, message.Kind)
		}
	}
	if page.Messages[0].Name != "first" || page.Messages[2].Name != "third" {
		t.Errorf("messages are not in payload order: %q … %q", page.Messages[0].Name, page.Messages[2].Name)
	}
	if page.NextCursor != "" {
		t.Errorf("a complete stream must not offer a next page, got cursor %q", page.NextCursor)
	}
}

func TestListEntryTranscript_PagesWithoutSkippingOrRepeating(t *testing.T) {
	entry := &entrydomain.Entry{
		ID: "entry-1", OrganizationID: "org-1", Type: "log",
		CreatedAt: fixedTestTime(),
		Message: `{"resourceLogs":[{"scopeLogs":[{"scope":{"name":"jobs"},"logRecords":[
			{"severityText":"one","timeUnixNano":"1700000000000000001"},
			{"severityText":"two","timeUnixNano":"1700000000000000002"},
			{"severityText":"three","timeUnixNano":"1700000000000000003"},
			{"severityText":"four","timeUnixNano":"1700000000000000004"},
			{"severityText":"five","timeUnixNano":"1700000000000000005"}
		]}]}]}`,
	}
	svc, records := newTranscriptSvc(entry)

	seen := make([]string, 0, 5)
	cursor := ""
	pages := 0
	for {
		page, err := svc.ListEntryTranscript(context.Background(), TranscriptQuery{
			EntryID: "entry-1", OrganizationID: "org-1", Cursor: cursor, Limit: 2,
		})
		if err != nil {
			t.Fatalf("page %d: %v", pages, err)
		}
		pages++
		for _, message := range page.Messages {
			seen = append(seen, message.Name)
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
		if pages > 10 {
			t.Fatal("pagination did not terminate")
		}
	}

	if strings.Join(seen, ",") != "one,two,three,four,five" {
		t.Errorf("paging lost or repeated messages: %v", seen)
	}
	if pages != 3 {
		t.Errorf("expected 3 pages of 2/2/1, got %d", pages)
	}
	if len(records.created) != 3 {
		t.Errorf("each page read must be audited once; got %d records", len(records.created))
	}
}

func TestListEntryTranscript_RedactsCredentialsServerSide(t *testing.T) {
	entry := &entrydomain.Entry{
		ID: "entry-1", OrganizationID: "org-1", Type: "trace",
		CreatedAt: fixedTestTime(),
		Message: `{"resourceSpans":[{"scopeSpans":[{"scope":{"name":"auth"},"spans":[{
			"name":"login",
			"attributes":[
				{"key":"http.request.header.authorization","value":{"stringValue":"credential-value-a"}},
				{"key":"db.password","value":{"stringValue":"credential-value-b"}},
				{"key":"http.request.header.cookie","value":{"stringValue":"credential-value-c"}},
				{"key":"user.email","value":{"stringValue":"ada@example.com"}}
			]}]}]}]}`,
	}
	svc, _ := newTranscriptSvc(entry)

	page, err := svc.ListEntryTranscript(context.Background(), TranscriptQuery{
		EntryID: "entry-1", OrganizationID: "org-1",
	})
	if err != nil {
		t.Fatalf("ListEntryTranscript: %v", err)
	}

	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	encoded := string(raw)

	for _, leaked := range []string{"credential-value-a", "credential-value-b", "credential-value-c"} {
		if strings.Contains(encoded, leaked) {
			t.Errorf("response leaked %q: %s", leaked, encoded)
		}
	}
	if !strings.Contains(encoded, "ada@example.com") {
		t.Errorf("non-sensitive payload was over-redacted: %s", encoded)
	}
	if !strings.Contains(encoded, redact.Placeholder) {
		t.Errorf("response carries no redaction marker: %s", encoded)
	}

	want := []string{
		"attributes.db.password",
		"attributes.http.request.header.authorization",
		"attributes.http.request.header.cookie",
	}
	if strings.Join(page.RedactedFields, ",") != strings.Join(want, ",") {
		t.Errorf("redactedFields = %v, want %v", page.RedactedFields, want)
	}
}

func TestListEntryTranscript_RefusesForeignTenantAsNotFound(t *testing.T) {
	entry := &entrydomain.Entry{
		ID: "entry-1", OrganizationID: "org-1", Type: "trace",
		CreatedAt: fixedTestTime(),
		Message:   `{"resourceSpans":[]}`,
	}
	svc, records := newTranscriptSvc(entry)

	_, err := svc.ListEntryTranscript(context.Background(), TranscriptQuery{
		EntryID: "entry-1", OrganizationID: "org-2",
	})

	assertNotFound(t, err)
	if len(records.created) != 0 {
		t.Error("a refused read must not be audited as a successful read")
	}
}

// TestListEntryTranscript_UsesTenantScopedRepositoryRead pins that the request
// path binds the tenant in the repository query: the system-scoped read must
// never be called, so a foreign row cannot be loaded at all.
func TestListEntryTranscript_UsesTenantScopedRepositoryRead(t *testing.T) {
	repo := &mockEntryRepo{entries: []*entrydomain.Entry{{
		ID: "entry-1", OrganizationID: "org-1", Type: "trace",
		CreatedAt: fixedTestTime(), Message: `{"resourceSpans":[]}`,
	}}}
	svc := Service{
		Clock: fakeClock{fixedTime: fixedTestTime()},
		UUID:  fakeUUIDGenerator{fixedID: "test-id"},
		UOW:   testUnitOfWork{entryRepo: repo, recordRepo: &mockRecordRepo{}},
	}

	if _, err := svc.ListEntryTranscript(context.Background(), TranscriptQuery{
		EntryID: "entry-1", OrganizationID: "org-1",
	}); err != nil {
		t.Fatalf("ListEntryTranscript: %v", err)
	}

	if repo.getByIDCalls != 0 {
		t.Errorf("transcript must not use the system-scoped GetByID; calls = %d", repo.getByIDCalls)
	}
	if repo.getByIDForOrganizationCalls != 1 {
		t.Errorf("transcript must use one tenant-scoped read; calls = %d", repo.getByIDForOrganizationCalls)
	}
}

func TestListEntryTranscript_MissingEntryIsNotFound(t *testing.T) {
	svc, _ := newTranscriptSvc(&entrydomain.Entry{ID: "entry-1", OrganizationID: "org-1", CreatedAt: fixedTestTime()})

	_, err := svc.ListEntryTranscript(context.Background(), TranscriptQuery{
		EntryID: "entry-does-not-exist", OrganizationID: "org-1",
	})

	assertNotFound(t, err)
}

func TestListEntryTranscript_CursorIsBoundToItsStreamAndOrder(t *testing.T) {
	entry := &entrydomain.Entry{
		ID: "entry-1", OrganizationID: "org-1", Type: "trace",
		CreatedAt: fixedTestTime(),
		Message: `{"resourceSpans":[{"scopeSpans":[{"scope":{"name":"web"},"spans":[
			{"name":"first"},{"name":"second"}
		]}]}]}`,
	}
	svc, _ := newTranscriptSvc(entry)

	first, err := svc.ListEntryTranscript(context.Background(), TranscriptQuery{
		EntryID: "entry-1", OrganizationID: "org-1", Limit: 1,
	})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if first.NextCursor == "" {
		t.Fatal("expected a cursor for the second page")
	}

	other := &entrydomain.Entry{
		ID: "entry-2", OrganizationID: "org-1", Type: "trace",
		CreatedAt: fixedTestTime(),
		Message:   `{"resourceSpans":[]}`,
	}
	svcOther, _ := newTranscriptSvc(other)
	_, err = svcOther.ListEntryTranscript(context.Background(), TranscriptQuery{
		EntryID: "entry-2", OrganizationID: "org-1", Cursor: first.NextCursor,
	})
	assertInvalidArgument(t, err)

	_, err = svc.ListEntryTranscript(context.Background(), TranscriptQuery{
		EntryID: "entry-1", OrganizationID: "org-1", Cursor: "not-a-cursor",
	})
	assertInvalidArgument(t, err)
}

func TestListEntryTranscript_RefusesOversizedLimitRatherThanTruncating(t *testing.T) {
	entry := &entrydomain.Entry{
		ID: "entry-1", OrganizationID: "org-1", Type: "trace",
		CreatedAt: fixedTestTime(), Message: `{"resourceSpans":[]}`,
	}
	svc, _ := newTranscriptSvc(entry)

	_, err := svc.ListEntryTranscript(context.Background(), TranscriptQuery{
		EntryID: "entry-1", OrganizationID: "org-1", Limit: MaxTranscriptLimit + 1,
	})
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("expected unprocessable entity, got %v", err)
	}
	if appErr.Meta["max"] != MaxTranscriptLimit {
		t.Fatalf("expected meta.max=%d, got %v", MaxTranscriptLimit, appErr.Meta)
	}
}

func TestListEntryTranscript_RefusesNegativeLimit(t *testing.T) {
	entry := &entrydomain.Entry{
		ID: "entry-1", OrganizationID: "org-1", Type: "trace",
		CreatedAt: fixedTestTime(), Message: `{"resourceSpans":[]}`,
	}
	svc, _ := newTranscriptSvc(entry)

	_, err := svc.ListEntryTranscript(context.Background(), TranscriptQuery{
		EntryID: "entry-1", OrganizationID: "org-1", Limit: -1,
	})
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("expected unprocessable entity, got %v", err)
	}
	if appErr.Meta["min"] != 0 {
		t.Fatalf("expected meta.min=0, got %v", appErr.Meta)
	}
}

func TestListEntryTranscript_UnreadablePayloadIsRefused(t *testing.T) {
	entry := &entrydomain.Entry{
		ID: "entry-1", OrganizationID: "org-1", Type: "trace",
		CreatedAt: fixedTestTime(), Message: `{ not json`,
	}
	svc, _ := newTranscriptSvc(entry)

	_, err := svc.ListEntryTranscript(context.Background(), TranscriptQuery{
		EntryID: "entry-1", OrganizationID: "org-1",
	})
	if err == nil {
		t.Fatal("an unreadable payload must not be served as an empty stream")
	}
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("expected unprocessable entity, got %v", err)
	}
}

func TestListEntryTranscript_EntriesWithoutASignalHaveNoMessages(t *testing.T) {
	entry := &entrydomain.Entry{
		ID: "entry-1", OrganizationID: "org-1", Type: "",
		CreatedAt: fixedTestTime(), Message: "a hand-written note",
	}
	svc, _ := newTranscriptSvc(entry)

	page, err := svc.ListEntryTranscript(context.Background(), TranscriptQuery{
		EntryID: "entry-1", OrganizationID: "org-1",
	})
	if err != nil {
		t.Fatalf("ListEntryTranscript: %v", err)
	}
	if len(page.Messages) != 0 || page.NextCursor != "" {
		t.Errorf("expected an empty, complete stream, got %+v", page)
	}
}

func TestListEntryTranscript_StampsRecordsWithoutAnEventTime(t *testing.T) {
	ingestedAt := fixedTestTime()
	entry := &entrydomain.Entry{
		ID: "entry-1", OrganizationID: "org-1", Type: "log",
		CreatedAt: ingestedAt,
		Message: `{"resourceLogs":[{"scopeLogs":[{"scope":{"name":"jobs"},"logRecords":[
			{"severityText":"no-ts"}
		]}]}]}`,
	}
	svc, _ := newTranscriptSvc(entry)

	page, err := svc.ListEntryTranscript(context.Background(), TranscriptQuery{
		EntryID: "entry-1", OrganizationID: "org-1",
	})
	if err != nil {
		t.Fatalf("ListEntryTranscript: %v", err)
	}

	if len(page.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(page.Messages))
	}
	if !page.Messages[0].Timestamp.Equal(ingestedAt.UTC()) {
		t.Errorf("a record without an event time must carry the ingestion time, got %s", page.Messages[0].Timestamp)
	}
	if page.Messages[0].Timestamp.IsZero() {
		t.Error("a message must never carry the zero time")
	}
}

func TestListEntryTranscript_AuditsTheRead(t *testing.T) {
	entry := &entrydomain.Entry{
		ID: "entry-1", OrganizationID: "org-1", Type: "trace",
		CreatedAt: fixedTestTime(),
		Message:   `{"resourceSpans":[{"scopeSpans":[{"scope":{"name":"web"},"spans":[{"name":"a"}]}]}]}`,
	}
	svc, records := newTranscriptSvc(entry)

	if _, err := svc.ListEntryTranscript(context.Background(), TranscriptQuery{
		EntryID: "entry-1", OrganizationID: "org-1",
		IPAddress: "203.0.113.7", UserAgent: "console/1.0",
		Surface: recordsdomain.SurfaceManagement, RequestID: "req-1", TraceID: "trace-1",
	}); err != nil {
		t.Fatalf("ListEntryTranscript: %v", err)
	}

	if len(records.created) != 1 {
		t.Fatalf("expected one audit row, got %d", len(records.created))
	}
	row := records.created[0]
	if row.Action != EntryTranscriptAction {
		t.Errorf("action = %q, want %q", row.Action, EntryTranscriptAction)
	}
	if row.OrganizationID != "org-1" || row.ResourceID != "entry-1" || row.ResourceType != "entry" {
		t.Errorf("audit row does not name what was read: %+v", row)
	}
	if row.IP != "203.0.113.7" || row.UserAgent != "console/1.0" || row.RequestID != "req-1" || row.TraceID != "trace-1" {
		t.Errorf("audit row lost request metadata: %+v", row)
	}
	if row.Surface != recordsdomain.SurfaceManagement {
		t.Errorf("surface = %q", row.Surface)
	}
}

func TestListEntryTranscript_RefusesEmptyIdentifiers(t *testing.T) {
	entry := &entrydomain.Entry{ID: "entry-1", OrganizationID: "org-1", CreatedAt: fixedTestTime()}
	svc, _ := newTranscriptSvc(entry)

	if _, err := svc.ListEntryTranscript(context.Background(), TranscriptQuery{OrganizationID: "org-1"}); err == nil {
		t.Error("an empty entry id must be refused")
	}
	if _, err := svc.ListEntryTranscript(context.Background(), TranscriptQuery{EntryID: "entry-1"}); err == nil {
		t.Error("a missing tenant scope must be refused rather than defaulted")
	}
}

func TestRedactTranscriptValue_NamesEveryRemovedPath(t *testing.T) {
	redacted := map[string]struct{}{}

	body := map[string]any{
		"Cookie":      "session=abc",
		"user.email":  "ada@example.com",
		"nested":      map[string]any{"clientSecret": "s3cret"},
		"tokenizer":   "readable",
		"userProfile": map[string]any{"tokenizerConfig": "readable"},
	}

	out := redactTranscriptValue("", body, "", redacted).(map[string]any)

	for _, path := range []string{"Cookie", "nested.clientSecret"} {
		if _, ok := redacted[path]; !ok {
			t.Errorf("path %q is not reported as redacted: %v", path, redacted)
		}
	}
	if _, ok := redacted["user.email"]; ok {
		t.Error("an innocuous field must not be reported as redacted")
	}
	if _, ok := redacted["tokenizer"]; ok {
		t.Error("a name that merely contains a sensitive word must stay readable")
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), "s3cret") || strings.Contains(string(encoded), "session=abc") {
		t.Errorf("credentials survived redaction: %s", encoded)
	}
	if !strings.Contains(string(encoded), "ada@example.com") || !strings.Contains(string(encoded), "tokenizerConfig") {
		t.Errorf("readable payload was over-redacted: %s", encoded)
	}
}

func TestRedactTranscriptValue_RedactsOtplAttributesByName(t *testing.T) {
	redacted := map[string]struct{}{}

	// The attribute's own `key` field names it: this is the shape every OTLP
	// KeyValue uses, and it is where instrumented request headers land.
	body := map[string]any{
		"attributes": []any{
			map[string]any{"key": "http.request.header.accept", "value": map[string]any{"stringValue": "application/json"}},
			map[string]any{"key": "http.request.header.x-api-key", "value": map[string]any{"stringValue": "credential-value-d"}},
			map[string]any{"key": "db.password", "value": map[string]any{"stringValue": "credential-value-e"}},
		},
	}

	out := redactTranscriptValue("", body, "", redacted).(map[string]any)
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(encoded), "credential-value-d") || strings.Contains(string(encoded), "credential-value-e") {
		t.Errorf("an attribute naming a credential survived redaction: %s", encoded)
	}
	if !strings.Contains(string(encoded), "application/json") {
		t.Errorf("an innocuous attribute was dropped: %s", encoded)
	}
	for _, path := range []string{"attributes.db.password", "attributes.http.request.header.x-api-key"} {
		if _, ok := redacted[path]; !ok {
			t.Errorf("path %q is not reported as redacted: %v", path, redacted)
		}
	}
	if len(redacted) != 2 {
		t.Errorf("only the credential attributes must be reported, got %v", redacted)
	}
}

func fixedTestTime() time.Time {
	return time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
}

func assertNotFound(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected the read to be refused")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected a platform error, got %T: %v", err, err)
	}
	if appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected 404 semantics, got %s (%v)", appErr.Code, err)
	}
}

func assertInvalidArgument(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected the request to be refused")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected a platform error, got %T: %v", err, err)
	}
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Fatalf("expected invalid-argument semantics, got %s (%v)", appErr.Code, err)
	}
}
