package infra

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	core "dappco.re/go/core"
)

// --- Constructor ---

func TestCloudNS_NewCloudNSClient_Good(t *testing.T) {
	c := NewCloudNSClient("12345", "secret")
	if c == nil {
		t.Fatal("expected non-nil")
	}
	if "12345" != c.authID {
		t.Fatalf("want %v, got %v", "12345", c.authID)
	}
	if "secret" != c.password {
		t.Fatalf("want %v, got %v", "secret", c.password)
	}
	if c.api == nil {
		t.Fatal("expected non-nil")
	}
}

// --- authParams ---

func TestCloudNS_CloudNSClient_AuthParams_Good(t *testing.T) {
	c := NewCloudNSClient("49500", "hunter2")
	params := c.authParams()
	if "49500" != params.Get("auth-id") {
		t.Fatalf("want %v, got %v", "49500", params.Get("auth-id"))
	}
	if "hunter2" != params.Get("auth-password") {
		t.Fatalf("want %v, got %v", "hunter2", params.Get("auth-password"))
	}
}

// --- doRaw ---

func TestCloudNS_CloudNSClient_DoRaw_ReturnsBody_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"Success"}`))
	}))
	defer ts.Close()

	client := NewCloudNSClient("test", "test")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{}),
	)

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/dns/test.json", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := client.doRaw(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(data), "Success") {
		t.Fatalf("expected %v to contain %v", string(data), "Success")
	}
}

func TestCloudNS_CloudNSClient_DoRaw_HTTPError_Bad(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"status":"Failed","statusDescription":"Invalid auth"}`))
	}))
	defer ts.Close()

	client := NewCloudNSClient("bad", "creds")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{}),
	)

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/dns/test.json", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.doRaw(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cloudns API: HTTP 403") {
		t.Fatalf("expected %v to contain %v", err.Error(), "cloudns API: HTTP 403")
	}
}

func TestCloudNS_CloudNSClient_DoRaw_ServerError_Bad(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`Internal Server Error`))
	}))
	defer ts.Close()

	client := NewCloudNSClient("test", "test")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{}),
	)

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.doRaw(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cloudns API: HTTP 500") {
		t.Fatalf("expected %v to contain %v", err.Error(), "cloudns API: HTTP 500")
	}
}

// --- Zone JSON parsing ---

func TestCloudNS_CloudNSZone_JSON_Good(t *testing.T) {
	data := `[
		{"name": "example.com", "type": "master", "zone": "domain", "status": "1"},
		{"name": "test.io", "type": "master", "zone": "domain", "status": "1"}
	]`

	var zones []CloudNSZone
	requireCloudNSJSON(t, data, &zones)
	if len(zones) != 2 {
		t.Fatalf("want length %v, got %v", 2, len(zones))
	}
	if "example.com" != zones[0].Name {
		t.Fatalf("want %v, got %v", "example.com", zones[0].Name)
	}
	if "master" != zones[0].Type {
		t.Fatalf("want %v, got %v", "master", zones[0].Type)
	}
	if "test.io" != zones[1].Name {
		t.Fatalf("want %v, got %v", "test.io", zones[1].Name)
	}
}

func TestCloudNS_CloudNSZone_JSON_EmptyResponse_Good(t *testing.T) {
	// CloudNS returns {} for no zones, not []
	data := `{}`

	var zones []CloudNSZone
	r := core.JSONUnmarshal([]byte(data), &zones)

	// Should fail to parse as slice — this is the edge case ListZones handles
	if r.OK {
		t.Fatal("expected false")
	}
}

// --- Record JSON parsing ---

func TestCloudNS_CloudNSRecord_JSON_Good(t *testing.T) {
	data := `{
		"12345": {
			"id": "12345",
			"type": "A",
			"host": "www",
			"record": "1.2.3.4",
			"ttl": "3600",
			"status": 1
		},
		"12346": {
			"id": "12346",
			"type": "MX",
			"host": "",
			"record": "mail.example.com",
			"ttl": "3600",
			"priority": "10",
			"status": 1
		}
	}`

	var records map[string]CloudNSRecord
	requireCloudNSJSON(t, data, &records)
	if len(records) != 2 {
		t.Fatalf("want length %v, got %v", 2, len(records))
	}

	aRecord := records["12345"]
	if "12345" != aRecord.ID {
		t.Fatalf("want %v, got %v", "12345", aRecord.ID)
	}
	if "A" != aRecord.Type {
		t.Fatalf("want %v, got %v", "A", aRecord.Type)
	}
	if "www" != aRecord.Host {
		t.Fatalf("want %v, got %v", "www", aRecord.Host)
	}
	if "1.2.3.4" != aRecord.Record {
		t.Fatalf("want %v, got %v", "1.2.3.4", aRecord.Record)
	}
	if "3600" != aRecord.TTL {
		t.Fatalf("want %v, got %v", "3600", aRecord.TTL)
	}
	if 1 != aRecord.Status {
		t.Fatalf("want %v, got %v", 1, aRecord.Status)
	}

	mxRecord := records["12346"]
	if "MX" != mxRecord.Type {
		t.Fatalf("want %v, got %v", "MX", mxRecord.Type)
	}
	if "mail.example.com" != mxRecord.Record {
		t.Fatalf("want %v, got %v", "mail.example.com", mxRecord.Record)
	}
	if "10" != mxRecord.Priority {
		t.Fatalf("want %v, got %v", "10", mxRecord.Priority)
	}
}

func TestCloudNS_CloudNSRecord_JSON_TXTRecord_Good(t *testing.T) {
	data := `{
		"99": {
			"id": "99",
			"type": "TXT",
			"host": "_acme-challenge",
			"record": "abc123def456",
			"ttl": "60",
			"status": 1
		}
	}`

	var records map[string]CloudNSRecord
	requireCloudNSJSON(t, data, &records)
	if len(records) != 1 {
		t.Fatalf("want length %v, got %v", 1, len(records))
	}

	txt := records["99"]
	if "TXT" != txt.Type {
		t.Fatalf("want %v, got %v", "TXT", txt.Type)
	}
	if "_acme-challenge" != txt.Host {
		t.Fatalf("want %v, got %v", "_acme-challenge", txt.Host)
	}
	if "abc123def456" != txt.Record {
		t.Fatalf("want %v, got %v", "abc123def456", txt.Record)
	}
	if "60" != txt.TTL {
		t.Fatalf("want %v, got %v", "60", txt.TTL)
	}
}

// --- CreateRecord response parsing ---

func TestCloudNS_CloudNSClient_CreateRecord_ResponseParsing_Good(t *testing.T) {
	data := `{"status":"Success","statusDescription":"The record was created successfully.","data":{"id":54321}}`

	var result struct {
		Status            string `json:"status"`
		StatusDescription string `json:"statusDescription"`
		Data              struct {
			ID int `json:"id"`
		} `json:"data"`
	}

	requireCloudNSJSON(t, data, &result)
	if "Success" != result.Status {
		t.Fatalf("want %v, got %v", "Success", result.Status)
	}
	if 54321 != result.Data.ID {
		t.Fatalf("want %v, got %v", 54321, result.Data.ID)
	}
}

func TestCloudNS_CloudNSClient_CreateRecord_FailedStatus_Bad(t *testing.T) {
	data := `{"status":"Failed","statusDescription":"Record already exists."}`

	var result struct {
		Status            string `json:"status"`
		StatusDescription string `json:"statusDescription"`
	}

	requireCloudNSJSON(t, data, &result)
	if "Failed" != result.Status {
		t.Fatalf("want %v, got %v", "Failed", result.Status)
	}
	if "Record already exists." != result.StatusDescription {
		t.Fatalf("want %v, got %v", "Record already exists.", result.StatusDescription)
	}
}

// --- UpdateRecord/DeleteRecord response parsing ---

func TestCloudNS_CloudNSClient_UpdateDelete_ResponseParsing_Good(t *testing.T) {
	data := `{"status":"Success","statusDescription":"The record was updated successfully."}`

	var result struct {
		Status            string `json:"status"`
		StatusDescription string `json:"statusDescription"`
	}

	requireCloudNSJSON(t, data, &result)
	if "Success" != result.Status {
		t.Fatalf("want %v, got %v", "Success", result.Status)
	}
}

// --- Full round-trip tests via doRaw ---

func TestCloudNS_CloudNSClient_ListZones_ViaDoRaw_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Query().Get("auth-id")) == 0 {
			t.Fatal("expected non-empty")
		}
		if len(r.URL.Query().Get("auth-password")) == 0 {
			t.Fatal("expected non-empty")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"example.com","type":"master","zone":"domain","status":"1"}]`))
	}))
	defer ts.Close()

	client := NewCloudNSClient("12345", "secret")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{}),
	)

	zones, err := client.ListZones(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(zones) != 1 {
		t.Fatalf("want length %v, got %v", 1, len(zones))
	}
	if "example.com" != zones[0].Name {
		t.Fatalf("want %v, got %v", "example.com", zones[0].Name)
	}
}

func TestCloudNS_CloudNSClient_ListRecords_ViaDoRaw_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "example.com" != r.URL.Query().Get("domain-name") {
			t.Fatalf("want %v, got %v", "example.com", r.URL.Query().Get("domain-name"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"1": {"id":"1","type":"A","host":"www","record":"1.2.3.4","ttl":"3600","status":1},
			"2": {"id":"2","type":"CNAME","host":"blog","record":"www.example.com","ttl":"3600","status":1}
		}`))
	}))
	defer ts.Close()

	client := NewCloudNSClient("12345", "secret")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{}),
	)

	records, err := client.ListRecords(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("want length %v, got %v", 2, len(records))
	}
	if "A" != records["1"].Type {
		t.Fatalf("want %v, got %v", "A", records["1"].Type)
	}
	if "CNAME" != records["2"].Type {
		t.Fatalf("want %v, got %v", "CNAME", records["2"].Type)
	}
}

func TestCloudNS_CloudNSClient_CreateRecord_ViaDoRaw_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if http.MethodPost != r.Method {
			t.Fatalf("want %v, got %v", http.MethodPost, r.Method)
		}
		if "example.com" != r.URL.Query().Get("domain-name") {
			t.Fatalf("want %v, got %v", "example.com", r.URL.Query().Get("domain-name"))
		}
		if "www" != r.URL.Query().Get("host") {
			t.Fatalf("want %v, got %v", "www", r.URL.Query().Get("host"))
		}
		if "A" != r.URL.Query().Get("record-type") {
			t.Fatalf("want %v, got %v", "A", r.URL.Query().Get("record-type"))
		}
		if "1.2.3.4" != r.URL.Query().Get("record") {
			t.Fatalf("want %v, got %v", "1.2.3.4", r.URL.Query().Get("record"))
		}
		if "3600" != r.URL.Query().Get("ttl") {
			t.Fatalf("want %v, got %v", "3600", r.URL.Query().Get("ttl"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"Success","statusDescription":"The record was created successfully.","data":{"id":99}}`))
	}))
	defer ts.Close()

	client := NewCloudNSClient("12345", "secret")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{}),
	)

	id, err := client.CreateRecord(context.Background(), "example.com", "www", "A", "1.2.3.4", 3600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if "99" != id {
		t.Fatalf("want %v, got %v", "99", id)
	}
}

func TestCloudNS_CloudNSClient_DeleteRecord_ViaDoRaw_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if http.MethodPost != r.Method {
			t.Fatalf("want %v, got %v", http.MethodPost, r.Method)
		}
		if "example.com" != r.URL.Query().Get("domain-name") {
			t.Fatalf("want %v, got %v", "example.com", r.URL.Query().Get("domain-name"))
		}
		if "42" != r.URL.Query().Get("record-id") {
			t.Fatalf("want %v, got %v", "42", r.URL.Query().Get("record-id"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"Success","statusDescription":"The record was deleted successfully."}`))
	}))
	defer ts.Close()

	client := NewCloudNSClient("12345", "secret")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{}),
	)

	err := client.DeleteRecord(context.Background(), "example.com", "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ACME challenge helpers ---

func TestCloudNS_CloudNSClient_SetACMEChallenge_ParamVerification_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "example.com" != r.URL.Query().Get("domain-name") {
			t.Fatalf("want %v, got %v", "example.com", r.URL.Query().Get("domain-name"))
		}
		if "_acme-challenge" != r.URL.Query().Get("host") {
			t.Fatalf("want %v, got %v", "_acme-challenge", r.URL.Query().Get("host"))
		}
		if "TXT" != r.URL.Query().Get("record-type") {
			t.Fatalf("want %v, got %v", "TXT", r.URL.Query().Get("record-type"))
		}
		if "60" != r.URL.Query().Get("ttl") {
			t.Fatalf("want %v, got %v", "60", r.URL.Query().Get("ttl"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"Success","statusDescription":"OK","data":{"id":777}}`))
	}))
	defer ts.Close()

	client := NewCloudNSClient("12345", "secret")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{}),
	)

	id, err := client.SetACMEChallenge(context.Background(), "example.com", "acme-token-value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if "777" != id {
		t.Fatalf("want %v, got %v", "777", id)
	}
}

func TestCloudNS_CloudNSClient_ClearACMEChallenge_Logic_Good(t *testing.T) {
	records := map[string]CloudNSRecord{
		"1": {ID: "1", Type: "A", Host: "www", Record: "1.2.3.4"},
		"2": {ID: "2", Type: "TXT", Host: "_acme-challenge", Record: "token1"},
		"3": {ID: "3", Type: "TXT", Host: "_dmarc", Record: "v=DMARC1"},
		"4": {ID: "4", Type: "TXT", Host: "_acme-challenge", Record: "token2"},
	}

	var toDelete []string
	for id, r := range records {
		if r.Host == "_acme-challenge" && r.Type == "TXT" {
			toDelete = append(toDelete, id)
		}
	}
	if len(toDelete) != 2 {
		t.Fatalf("want length %v, got %v", 2, len(toDelete))
	}
	if !slices.Contains(toDelete, "2") {
		t.Fatalf("expected %v to contain %v", toDelete, "2")
	}
	if !slices.Contains(toDelete, "4") {
		t.Fatalf("expected %v to contain %v", toDelete, "4")
	}
}

// --- EnsureRecord logic ---

func TestCloudNS_EnsureRecord_Logic_AlreadyCorrect_Good(t *testing.T) {
	records := map[string]CloudNSRecord{
		"10": {ID: "10", Type: "A", Host: "www", Record: "1.2.3.4"},
	}

	host := "www"
	recordType := "A"
	value := "1.2.3.4"

	var needsUpdate, needsCreate bool
	for _, r := range records {
		if r.Host == host && r.Type == recordType {
			if r.Record == value {
				needsUpdate = false
				needsCreate = false
			} else {
				needsUpdate = true
			}
			break
		}
	}

	if !needsUpdate {
		found := false
		for _, r := range records {
			if r.Host == host && r.Type == recordType {
				found = true
				break
			}
		}
		if !found {
			needsCreate = true
		}
	}
	if needsUpdate {
		t.Fatal("expected false")
	}
	if needsCreate {
		t.Fatal("expected false")
	}
}

func TestCloudNS_EnsureRecord_Logic_NeedsUpdate_Good(t *testing.T) {
	records := map[string]CloudNSRecord{
		"10": {ID: "10", Type: "A", Host: "www", Record: "1.2.3.4"},
	}

	host := "www"
	recordType := "A"
	value := "5.6.7.8"

	var needsUpdate bool
	for _, r := range records {
		if r.Host == host && r.Type == recordType {
			if r.Record != value {
				needsUpdate = true
			}
			break
		}
	}
	if !needsUpdate {
		t.Fatal("expected true")
	}
}

func TestCloudNS_EnsureRecord_Logic_NeedsCreate_Good(t *testing.T) {
	records := map[string]CloudNSRecord{
		"10": {ID: "10", Type: "A", Host: "www", Record: "1.2.3.4"},
	}

	host := "api"
	recordType := "A"

	found := false
	for _, r := range records {
		if r.Host == host && r.Type == recordType {
			found = true
			break
		}
	}
	if found {
		t.Fatal("expected false")
	}
}

// --- Edge cases ---

func TestCloudNS_CloudNSClient_DoRaw_EmptyBody_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewCloudNSClient("test", "test")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{}),
	)

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := client.doRaw(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty, got %v", data)
	}
}

func TestCloudNS_CloudNSRecord_JSON_EmptyMap_Good(t *testing.T) {
	data := `{}`

	var records map[string]CloudNSRecord
	requireCloudNSJSON(t, data, &records)
	if len(records) != 0 {
		t.Fatalf("expected empty, got %v", records)
	}
}

func requireCloudNSJSON(t *testing.T, data string, target any) {
	t.Helper()

	r := core.JSONUnmarshal([]byte(data), target)
	if !r.OK {
		t.Fatal("expected true")
	}
}

// --- UpdateRecord round-trip ---

func TestCloudNS_CloudNSClient_UpdateRecord_ViaDoRaw_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if http.MethodPost != r.Method {
			t.Fatalf("want %v, got %v", http.MethodPost, r.Method)
		}
		if "example.com" != r.URL.Query().Get("domain-name") {
			t.Fatalf("want %v, got %v", "example.com", r.URL.Query().Get("domain-name"))
		}
		if "42" != r.URL.Query().Get("record-id") {
			t.Fatalf("want %v, got %v", "42", r.URL.Query().Get("record-id"))
		}
		if "www" != r.URL.Query().Get("host") {
			t.Fatalf("want %v, got %v", "www", r.URL.Query().Get("host"))
		}
		if "A" != r.URL.Query().Get("record-type") {
			t.Fatalf("want %v, got %v", "A", r.URL.Query().Get("record-type"))
		}
		if "5.6.7.8" != r.URL.Query().Get("record") {
			t.Fatalf("want %v, got %v", "5.6.7.8", r.URL.Query().Get("record"))
		}
		if "3600" != r.URL.Query().Get("ttl") {
			t.Fatalf("want %v, got %v", "3600", r.URL.Query().Get("ttl"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"Success","statusDescription":"The record was updated successfully."}`))
	}))
	defer ts.Close()

	client := NewCloudNSClient("12345", "secret")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{}),
	)

	err := client.UpdateRecord(context.Background(), "example.com", "42", "www", "A", "5.6.7.8", 3600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCloudNS_CloudNSClient_UpdateRecord_FailedStatus_Bad(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"Failed","statusDescription":"Record not found."}`))
	}))
	defer ts.Close()

	client := NewCloudNSClient("12345", "secret")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{}),
	)

	err := client.UpdateRecord(context.Background(), "example.com", "999", "www", "A", "5.6.7.8", 3600)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Record not found") {
		t.Fatalf("expected %v to contain %v", err.Error(), "Record not found")
	}
}

// --- EnsureRecord round-trip ---

func TestCloudNS_CloudNSClient_EnsureRecord_AlreadyCorrect_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"1":{"id":"1","type":"A","host":"www","record":"1.2.3.4","ttl":"3600","status":1}}`))
	}))
	defer ts.Close()

	client := NewCloudNSClient("12345", "secret")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{}),
	)

	changed, err := client.EnsureRecord(context.Background(), "example.com", "www", "A", "1.2.3.4", 3600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Fatal("expected false")
	}
}

func TestCloudNS_CloudNSClient_EnsureRecord_NeedsUpdate_Good(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// ListRecords — returns existing record with old value
			_, _ = w.Write([]byte(`{"1":{"id":"1","type":"A","host":"www","record":"1.2.3.4","ttl":"3600","status":1}}`))
		} else {
			// UpdateRecord
			if http.MethodPost != r.Method {
				t.Fatalf("want %v, got %v", http.MethodPost, r.Method)
			}
			if "5.6.7.8" != r.URL.Query().Get("record") {
				t.Fatalf("want %v, got %v", "5.6.7.8", r.URL.Query().Get("record"))
			}
			_, _ = w.Write([]byte(`{"status":"Success","statusDescription":"The record was updated successfully."}`))
		}
	}))
	defer ts.Close()

	client := NewCloudNSClient("12345", "secret")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{}),
	)

	changed, err := client.EnsureRecord(context.Background(), "example.com", "www", "A", "5.6.7.8", 3600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("expected true")
	}
}

func TestCloudNS_CloudNSClient_EnsureRecord_NeedsCreate_Good(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// ListRecords — no matching record
			_, _ = w.Write([]byte(`{"1":{"id":"1","type":"A","host":"other","record":"1.2.3.4","ttl":"3600","status":1}}`))
		} else {
			// CreateRecord
			if http.MethodPost != r.Method {
				t.Fatalf("want %v, got %v", http.MethodPost, r.Method)
			}
			if "www" != r.URL.Query().Get("host") {
				t.Fatalf("want %v, got %v", "www", r.URL.Query().Get("host"))
			}
			_, _ = w.Write([]byte(`{"status":"Success","statusDescription":"The record was created successfully.","data":{"id":99}}`))
		}
	}))
	defer ts.Close()

	client := NewCloudNSClient("12345", "secret")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{}),
	)

	changed, err := client.EnsureRecord(context.Background(), "example.com", "www", "A", "1.2.3.4", 3600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("expected true")
	}
}

// --- ClearACMEChallenge round-trip ---

func TestCloudNS_CloudNSClient_ClearACMEChallenge_ViaDoRaw_Good(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// ListRecords — returns ACME challenge records
			_, _ = w.Write([]byte(`{
				"1":{"id":"1","type":"A","host":"www","record":"1.2.3.4","ttl":"3600","status":1},
				"2":{"id":"2","type":"TXT","host":"_acme-challenge","record":"token1","ttl":"60","status":1}
			}`))
		} else {
			// DeleteRecord
			if http.MethodPost != r.Method {
				t.Fatalf("want %v, got %v", http.MethodPost, r.Method)
			}
			if "2" != r.URL.Query().Get("record-id") {
				t.Fatalf("want %v, got %v", "2", r.URL.Query().Get("record-id"))
			}
			_, _ = w.Write([]byte(`{"status":"Success","statusDescription":"The record was deleted successfully."}`))
		}
	}))
	defer ts.Close()

	client := NewCloudNSClient("12345", "secret")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{}),
	)

	err := client.ClearACMEChallenge(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount < 2 {
		t.Fatalf("want >= %v, got %v", 2, callCount)
	}
}

func TestCloudNS_CloudNSClient_DoRaw_AuthQueryParams_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "49500" != r.URL.Query().Get("auth-id") {
			t.Fatalf("want %v, got %v", "49500", r.URL.Query().Get("auth-id"))
		}
		if "supersecret" != r.URL.Query().Get("auth-password") {
			t.Fatalf("want %v, got %v", "supersecret", r.URL.Query().Get("auth-password"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	client := NewCloudNSClient("49500", "supersecret")
	client.baseURL = ts.URL
	client.api = NewAPIClient(
		WithHTTPClient(ts.Client()),
		WithPrefix("cloudns API"),
		WithRetry(RetryConfig{}),
	)

	ctx := context.Background()
	params := client.authParams()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/dns/test.json?"+params.Encode(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.doRaw(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
