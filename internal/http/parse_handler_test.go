package http

import (
	"context"
	"errors"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"finance-parser-go/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/xeipuuv/gojsonschema"
)

type fixtureParser struct {
	result []byte
	err    error
}

func (p fixtureParser) Transcribe(context.Context, string, []byte) (string, error) {
	return "", nil
}

func (p fixtureParser) ParseText(context.Context, string, string) ([]byte, error) {
	return p.result, p.err
}

func TestParseHandlerMapsProviderFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := &Server{
		cfg:    &config.Config{ReqTimeoutSec: 2, TZDefault: "Asia/Kolkata"},
		parser: fixtureParser{err: errors.New("provider unavailable")},
	}
	form := url.Values{"hint_text": {"chai ke 80 rupaye"}}
	request := httptest.NewRequest("POST", "/v1/parse", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = request

	server.handleParse(context)

	if response.Code != 422 || !strings.Contains(response.Body.String(), "could_not_parse") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestParseHandlerReturnsDraftWithoutDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	schema, err := gojsonschema.NewSchema(
		gojsonschema.NewReferenceLoader("file://../../schemas/expense_entry.schema.json"),
	)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{
		cfg:       &config.Config{ReqTimeoutSec: 2, TZDefault: "Asia/Kolkata"},
		validator: schema,
		parser: fixtureParser{result: []byte(`{
			"title":"Metro","amount":45,"type":"expense","currency":"INR",
			"mode":"UPI","category":"Travel","date":"2026-07-09"
		}`)},
	}

	form := url.Values{"hint_text": {"metro 45 via upi"}}
	request := httptest.NewRequest("POST", "/v1/parse", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = request

	// database.DB intentionally remains nil. A successful response proves the
	// parse path has no transaction-persistence dependency.
	server.handleParse(context)

	if response.Code != 200 {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), `"id"`) {
		t.Fatalf("parse response unexpectedly looks persisted: %s", response.Body.String())
	}
}
