package ssf

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	ssfdomain "github.com/guliangiam/guliangiam/internal/domain/ssf"
	mysqlssf "github.com/guliangiam/guliangiam/internal/infra/mysql/ssf"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandlerDeliversThroughMySQLReceiverRegistry proves SSF delivery works
// when the receiver registry is backed by MySQL (OPEN-API-74).
func TestHandlerDeliversThroughMySQLReceiverRegistry(t *testing.T) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	const orgID = "01a0ea76-0c00-7000-8000-000000000001"
	_, err = db.ExecContext(ctx, `INSERT IGNORE INTO organizations (id, slug, display_name, revision) VALUES (?, 'built-in', 'Built-in', 1)`, orgID)
	require.NoError(t, err)

	received := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- string(body)
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)

	repo := mysqlssf.NewReceiverRepository(db)
	receiver := &ssfdomain.Receiver{
		ID: uuid.NewString(), OrganizationID: orgID, Name: "mysql-" + uuid.NewString(),
		ReceiverURL: server.URL, EventTypes: []byte(`["*"]`), Status: "active",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Revision: 1,
	}
	require.NoError(t, repo.Create(ctx, receiver))
	t.Cleanup(func() { _ = repo.Delete(context.Background(), orgID, receiver.ID) })

	headers, err := json.Marshal(map[string]string{"receiver_id": receiver.ID, "organization_id": orgID})
	require.NoError(t, err)
	payload, err := json.Marshal(map[string]any{
		"type": "session-revoked", "issuer": "https://op.example.com", "eventId": "evt-1",
	})
	require.NoError(t, err)
	handler := NewHandler(repo, &fakeSigner{token: "signed-token"}, server.Client())
	require.NoError(t, handler.Handle(ctx, &platformoutbox.OutboxItem{Headers: headers, Payload: payload}))
	select {
	case body := <-received:
		assert.Equal(t, "signed-token", body)
	case <-time.After(2 * time.Second):
		t.Fatal("receiver did not get the SSF event")
	}
}
