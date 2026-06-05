package accounts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/go-chi/chi/v5"
)

// createPublisherJSON mirrors the decode struct in createPublisher.
type createPublisherJSON struct {
	Name           string `json:"name"`
	Website        string `json:"website"`
	Timezone       string `json:"timezone"`
	AdminEmail     string `json:"admin_email"`
	AdminFirstName string `json:"admin_first_name"`
	AdminLastName  string `json:"admin_last_name"`
}

func TestCreatePublisherJSON_decode(t *testing.T) {
	const raw = `{
		"name": "Affiniti",
		"admin_first_name": "Eugene",
		"admin_last_name": "Chayko",
		"admin_email": "evgeny.chayko@example.com"
	}`
	var body createPublisherJSON
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	p := CreatePublisherParams{
		Name:           body.Name,
		Website:        body.Website,
		Timezone:       body.Timezone,
		AdminEmail:     body.AdminEmail,
		AdminFirstName: body.AdminFirstName,
		AdminLastName:  body.AdminLastName,
	}
	if p.Name != "Affiniti" || p.AdminFirstName != "Eugene" || p.AdminLastName != "Chayko" || p.AdminEmail != "evgeny.chayko@example.com" {
		t.Fatalf("got params %+v", p)
	}
}

func TestCreatePublisher_handler(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	h := NewHandler(NewService(NewRepository(pool), nil, nil, nil))
	email := fmt.Sprintf("pub-create-%d@example.com", time.Now().UnixNano())
	body := fmt.Sprintf(
		`{"name":"Affiniti Test","admin_first_name":"Eugene","admin_last_name":"Chayko","admin_email":%q}`,
		email,
	)
	req := httptest.NewRequest(http.MethodPost, "/platform/publishers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.createPublisher(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPatchPublisher_handler(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	h := NewHandler(NewService(NewRepository(pool), nil, nil, nil))

	createBody := fmt.Sprintf(
		`{"name":"Patch Test Pub","admin_first_name":"A","admin_last_name":"B","admin_email":%q}`,
		fmt.Sprintf("patch-pub-%d@example.com", time.Now().UnixNano()),
	)
	createReq := httptest.NewRequest(http.MethodPost, "/platform/publishers", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	h.createPublisher(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", createRec.Code, createRec.Body.String())
	}

	var createResp struct {
		Data Account `json:"data"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create: %v body=%s", err, createRec.Body.String())
	}
	created := createResp.Data

	patchReq := httptest.NewRequest(
		http.MethodPatch,
		"/platform/publishers/"+created.PublicID,
		strings.NewReader(`{"name":"Renamed Pub","operational_status":"suspended"}`),
	)
	patchReq.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", created.PublicID)
	patchReq = patchReq.WithContext(context.WithValue(patchReq.Context(), chi.RouteCtxKey, rctx))
	patchRec := httptest.NewRecorder()
	h.patchPublisherStatus(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch status %d: %s", patchRec.Code, patchRec.Body.String())
	}

	var patchResp struct {
		Data Account `json:"data"`
	}
	if err := json.Unmarshal(patchRec.Body.Bytes(), &patchResp); err != nil {
		t.Fatalf("decode patch: %v", err)
	}
	updated := patchResp.Data
	if updated.Name != "Renamed Pub" || updated.OperationalStatus != AccountStatusSuspended {
		t.Fatalf("got %+v", updated)
	}
}

func TestDeletePublisher_handler(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Connect(ctx, "postgres://crm:crm@localhost:5432/crm?sslmode=disable")
	if err != nil {
		t.Skip(err)
	}
	defer pool.Close()

	h := NewHandler(NewService(NewRepository(pool), nil, nil, nil))

	createBody := fmt.Sprintf(
		`{"name":"Delete Test Pub","admin_first_name":"A","admin_last_name":"B","admin_email":%q}`,
		fmt.Sprintf("delete-pub-%d@example.com", time.Now().UnixNano()),
	)
	createReq := httptest.NewRequest(http.MethodPost, "/platform/publishers", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	h.createPublisher(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", createRec.Code, createRec.Body.String())
	}

	var createResp struct {
		Data Account `json:"data"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	created := createResp.Data

	delReq := httptest.NewRequest(http.MethodDelete, "/platform/publishers/"+created.PublicID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", created.PublicID)
	delReq = delReq.WithContext(context.WithValue(delReq.Context(), chi.RouteCtxKey, rctx))
	delRec := httptest.NewRecorder()
	h.deletePublisher(delRec, delReq)
	if delRec.Code != http.StatusOK {
		t.Fatalf("delete status %d: %s", delRec.Code, delRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/platform/publishers?page=1&limit=100", nil)
	listRec := httptest.NewRecorder()
	h.listPublishers(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status %d: %s", listRec.Code, listRec.Body.String())
	}

	var listResp struct {
		Data struct {
			Items []Account `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	for _, item := range listResp.Data.Items {
		if item.PublicID == created.PublicID {
			t.Fatalf("deleted publisher still in list: %+v", item)
		}
	}
}
