package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// matrixTxnCounter backs the transaction ID suffix. Matrix deduplicates sends
// per session by txnId, so every call must produce a unique value — even when
// the caller context carries a request ID (one request can fan out to several
// notifications).
var matrixTxnCounter atomic.Uint64

func matrixTxnID(ctx context.Context) string {
	if v := ctx.Value("requestID"); v != nil {
		if s := fmt.Sprintf("%v", v); s != "" && s != "<nil>" {
			return "guliangiam-" + s + "-" + strconv.FormatUint(matrixTxnCounter.Add(1), 10)
		}
	}
	return "guliangiam-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + strconv.FormatUint(matrixTxnCounter.Add(1), 10)
}

// MatrixNotifier sends notifications via Matrix Client API.
//
// Calibration (OPEN-API-75 P6, 2026-11-14, 已按官方规范复核，无偏差)：
// PUT /_matrix/client/v3/rooms/{roomId}/send/m.room.message/{txnId} +
// Bearer access_token，路径版本与事务语义见
// https://spec.matrix.org/v1.11/client-server-api/#put_matrixclientv3roomsroomidsendeventtypetxnid
type MatrixNotifier struct {
	HomeServer  string
	AccessToken string
	HTTPClient  *http.Client
}

// NewMatrixNotifier creates a new Matrix notifier.
func NewMatrixNotifier(homeServer, accessToken string) *MatrixNotifier {
	return &MatrixNotifier{
		HomeServer:  homeServer,
		AccessToken: accessToken,
		HTTPClient:  ssrf.DefaultHTTPClient,
	}
}

// matrixPayload represents the JSON payload sent to Matrix Client API.
type matrixPayload struct {
	MsgType string `json:"msgtype"`
	Body    string `json:"body"`
}

func (n *MatrixNotifier) Send(ctx context.Context, notif *Notification) error {
	if n.HomeServer == "" {
		return apperrors.NewInvalidArgument("matrix notification: home server is required")
	}
	if n.AccessToken == "" {
		return apperrors.NewInvalidArgument("matrix notification: access token is required")
	}
	if notif.Recipient == "" {
		return apperrors.NewInvalidArgument("matrix notification: room ID is required")
	}

	// Matrix Client API endpoint: PUT /_matrix/client/v3/rooms/{roomId}/send/m.room.message/{txnId}
	txnID := matrixTxnID(ctx)
	apiURL := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/send/m.room.message/%s",
		strings.TrimRight(n.HomeServer, "/"), url.PathEscape(notif.Recipient), url.PathEscape(txnID))

	payload := matrixPayload{
		MsgType: "m.text",
		Body:    notif.Content,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.WrapInternalf(err, "matrix notification: marshal payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, bytes.NewReader(body))
	if err != nil {
		return apperrors.WrapInternalf(err, "matrix notification: create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+n.AccessToken)

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return apperrors.WrapInternalf(err, "matrix notification: send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return apperrors.NewInternalf("matrix notification: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
