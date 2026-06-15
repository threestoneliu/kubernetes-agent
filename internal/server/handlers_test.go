package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

func TestListPoliciesHandler_Success(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/policies")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Policies []policyView `json:"policies"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	// Default policies are seeded by Migrate.
	assert.NotEmpty(t, body.Policies)
}

func TestUpdatePolicyHandler_InvalidJSON(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/policies/p1", bytes.NewReader([]byte(`not json`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdatePolicyHandler_EmptyYAML(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/policies/p1", bytes.NewReader([]byte(`{"yaml":""}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdatePolicyHandler_InvalidYAML(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/policies/p1", bytes.NewReader([]byte(`{"yaml":"not yaml: [[["}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdatePolicyHandler_MissingName(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	body := bytes.NewReader([]byte(`{"yaml":"effect: allow\n"}`))
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/policies/p1", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdatePolicyHandler_MissingEffect(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	body := bytes.NewReader([]byte(`{"yaml":"name: foo\n"}`))
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/policies/p1", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUpdatePolicyHandler_PolicyNotFound(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	body := bytes.NewReader([]byte(`{"yaml":"name: x\neffect: allow\n"}`))
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/policies/missing", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUpdatePolicyHandler_Success(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	policies, err := d.DB.ListAllPolicies(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, policies)
	p := policies[0]

	body := bytes.NewReader([]byte(`{"yaml":"name: ` + p.Name + `\neffect: allow\nmatch:\n  action: [\"get\"]\n"}`))
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/policies/"+p.ID, body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTogglePolicyHandler_InvalidJSON(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/policies/p1/enabled", bytes.NewReader([]byte(`not json`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTogglePolicyHandler_NotFound(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/policies/missing/enabled", bytes.NewReader([]byte(`{"enabled":true}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestTogglePolicyHandler_Success(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	policies, err := d.DB.ListAllPolicies(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, policies)
	p := policies[0]

	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/policies/"+p.ID+"/enabled", bytes.NewReader([]byte(`{"enabled":false}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestSessions_CreateAndGet(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/sessions", "application/json", bytes.NewReader([]byte(`{"title":"test"}`)))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created sessionView
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	assert.Equal(t, "test", created.Title)

	resp, err = http.Get(ts.URL + "/api/sessions/" + created.ID)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGetSessionHandler_NotFound(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/sessions/missing")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestListSessionsHandler_Empty(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/sessions")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestListMessagesHandler_NotFound(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/sessions/missing/messages")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestListMessagesHandler_Success(t *testing.T) {
	d := testDeps(t)
	ts := httptest.NewServer(NewRouter(d))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/sessions", "application/json", bytes.NewReader([]byte(`{"title":"m"}`)))
	require.NoError(t, err)
	defer resp.Body.Close()
	var s sessionView
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&s))

	resp, err = http.Get(ts.URL + "/api/sessions/" + s.ID + "/messages")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestToPolicyView(t *testing.T) {
	now := time.Now()
	v := toPolicyView(store.Policy{
		ID: "p1", Name: "name", YAML: "yaml", Enabled: true,
		CreatedAt: now, UpdatedAt: now,
	})
	assert.Equal(t, "p1", v.ID)
	assert.Equal(t, "name", v.Name)
	assert.True(t, v.Enabled)
}

func TestToSessionView(t *testing.T) {
	now := time.Now()
	v := toSessionView(store.Session{ID: "s1", Title: "title", CreatedAt: now, UpdatedAt: now})
	assert.Equal(t, "s1", v.ID)
	assert.Equal(t, "title", v.Title)
}

func TestToMessageView(t *testing.T) {
	v := toMessageView(store.Message{ID: "m1", Role: "user", CreatedAt: 1})
	assert.Equal(t, "m1", v.ID)
	assert.Equal(t, "user", v.Role)
}

func TestGetPolicyByID_Found(t *testing.T) {
	d := testDeps(t)
	require.NoError(t, d.DB.UpsertPolicy(t.Context(), store.Policy{
		ID: "p1", Name: "x", YAML: "name: x\neffect: allow\n", Enabled: true,
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	got, err := getPolicyByID(r, d, "p1")
	require.NoError(t, err)
	assert.Equal(t, "p1", got.ID)
}

func TestGetPolicyByID_NotFound(t *testing.T) {
	d := testDeps(t)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := getPolicyByID(r, d, "missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, store.ErrNotFound))
}

func TestPolicyLookupError_NotFound(t *testing.T) {
	w := httptest.NewRecorder()
	policyLookupError(w, store.ErrNotFound)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPolicyLookupError_Internal(t *testing.T) {
	w := httptest.NewRecorder()
	policyLookupError(w, errors.New("db down"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestWriteAudit_NilDB(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	writeAudit(r, Deps{}, "x", "y") // no DB — no panic
}