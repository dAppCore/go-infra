package infra

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	core "dappco.re/go/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Constructor ---

func TestCloudNS_NewCloudNSClient_Good(t *testing.T) {
	c := NewCloudNSClient("12345", "secret")
	assert.NotNil(t, c)
	assert.Equal(t, "12345", c.authID)
	assert.Equal(t, "secret", c.password)
	assert.NotNil(t, c.api)
}

// --- authParams ---

func TestCloudNS_CloudNSClient_AuthParams_Good(t *testing.T) {
	c := NewCloudNSClient("49500", "hunter2")
	params := c.authParams()

	assert.Equal(t, "49500", params.Get("auth-id"))
	assert.Equal(t, "hunter2", params.Get("auth-password"))
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
	require.NoError(t, err)

	data, err := client.doRaw(req)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Success")
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
	require.NoError(t, err)

	_, err = client.doRaw(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cloudns API: HTTP 403")
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
	require.NoError(t, err)

	_, err = client.doRaw(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cloudns API: HTTP 500")
}

// --- Zone JSON parsing ---

func TestCloudNS_CloudNSZone_JSON_Good(t *testing.T) {
	data := `[
		{"name": "example.com", "type": "master", "zone": "domain", "status": "1"},
		{"name": "test.io", "type": "master", "zone": "domain", "status": "1"}
	]`

	var zones []CloudNSZone
	requireCloudNSJSON(t, data, &zones)
	require.Len(t, zones, 2)
	assert.Equal(t, "example.com", zones[0].Name)
	assert.Equal(t, "master", zones[0].Type)
	assert.Equal(t, "test.io", zones[1].Name)
}

func TestCloudNS_CloudNSZone_JSON_EmptyResponse_Good(t *testing.T) {
	// CloudNS returns {} for no zones, not []
	data := `{}`

	var zones []CloudNSZone
	r := core.JSONUnmarshal([]byte(data), &zones)

	// Should fail to parse as slice — this is the edge case ListZones handles
	assert.False(t, r.OK)
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
	require.Len(t, records, 2)

	aRecord := records["12345"]
	assert.Equal(t, "12345", aRecord.ID)
	assert.Equal(t, "A", aRecord.Type)
	assert.Equal(t, "www", aRecord.Host)
	assert.Equal(t, "1.2.3.4", aRecord.Record)
	assert.Equal(t, "3600", aRecord.TTL)
	assert.Equal(t, 1, aRecord.Status)

	mxRecord := records["12346"]
	assert.Equal(t, "MX", mxRecord.Type)
	assert.Equal(t, "mail.example.com", mxRecord.Record)
	assert.Equal(t, "10", mxRecord.Priority)
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
	require.Len(t, records, 1)

	txt := records["99"]
	assert.Equal(t, "TXT", txt.Type)
	assert.Equal(t, "_acme-challenge", txt.Host)
	assert.Equal(t, "abc123def456", txt.Record)
	assert.Equal(t, "60", txt.TTL)
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
	assert.Equal(t, "Success", result.Status)
	assert.Equal(t, 54321, result.Data.ID)
}

func TestCloudNS_CloudNSClient_CreateRecord_FailedStatus_Bad(t *testing.T) {
	data := `{"status":"Failed","statusDescription":"Record already exists."}`

	var result struct {
		Status            string `json:"status"`
		StatusDescription string `json:"statusDescription"`
	}

	requireCloudNSJSON(t, data, &result)
	assert.Equal(t, "Failed", result.Status)
	assert.Equal(t, "Record already exists.", result.StatusDescription)
}

// --- UpdateRecord/DeleteRecord response parsing ---

func TestCloudNS_CloudNSClient_UpdateDelete_ResponseParsing_Good(t *testing.T) {
	data := `{"status":"Success","statusDescription":"The record was updated successfully."}`

	var result struct {
		Status            string `json:"status"`
		StatusDescription string `json:"statusDescription"`
	}

	requireCloudNSJSON(t, data, &result)
	assert.Equal(t, "Success", result.Status)
}

// --- Full round-trip tests via doRaw ---

func TestCloudNS_CloudNSClient_ListZones_ViaDoRaw_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NotEmpty(t, r.URL.Query().Get("auth-id"))
		assert.NotEmpty(t, r.URL.Query().Get("auth-password"))

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
	require.NoError(t, err)
	require.Len(t, zones, 1)
	assert.Equal(t, "example.com", zones[0].Name)
}

func TestCloudNS_CloudNSClient_ListRecords_ViaDoRaw_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "example.com", r.URL.Query().Get("domain-name"))

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
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, "A", records["1"].Type)
	assert.Equal(t, "CNAME", records["2"].Type)
}

func TestCloudNS_CloudNSClient_CreateRecord_ViaDoRaw_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "example.com", r.URL.Query().Get("domain-name"))
		assert.Equal(t, "www", r.URL.Query().Get("host"))
		assert.Equal(t, "A", r.URL.Query().Get("record-type"))
		assert.Equal(t, "1.2.3.4", r.URL.Query().Get("record"))
		assert.Equal(t, "3600", r.URL.Query().Get("ttl"))

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
	require.NoError(t, err)
	assert.Equal(t, "99", id)
}

func TestCloudNS_CloudNSClient_DeleteRecord_ViaDoRaw_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "example.com", r.URL.Query().Get("domain-name"))
		assert.Equal(t, "42", r.URL.Query().Get("record-id"))

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
	require.NoError(t, err)
}

// --- ACME challenge helpers ---

func TestCloudNS_CloudNSClient_SetACMEChallenge_ParamVerification_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "example.com", r.URL.Query().Get("domain-name"))
		assert.Equal(t, "_acme-challenge", r.URL.Query().Get("host"))
		assert.Equal(t, "TXT", r.URL.Query().Get("record-type"))
		assert.Equal(t, "60", r.URL.Query().Get("ttl"))

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
	require.NoError(t, err)
	assert.Equal(t, "777", id)
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

	assert.Len(t, toDelete, 2)
	assert.Contains(t, toDelete, "2")
	assert.Contains(t, toDelete, "4")
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

	assert.False(t, needsUpdate, "should not need update when value matches")
	assert.False(t, needsCreate, "should not need create when record exists")
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

	assert.True(t, needsUpdate, "should need update when value differs")
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

	assert.False(t, found, "should not find record for non-existent host")
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
	require.NoError(t, err)

	data, err := client.doRaw(req)
	require.NoError(t, err)
	assert.Empty(t, data)
}

func TestCloudNS_CloudNSRecord_JSON_EmptyMap_Good(t *testing.T) {
	data := `{}`

	var records map[string]CloudNSRecord
	requireCloudNSJSON(t, data, &records)
	assert.Empty(t, records)
}

func requireCloudNSJSON(t *testing.T, data string, target any) {
	t.Helper()

	r := core.JSONUnmarshal([]byte(data), target)
	require.True(t, r.OK)
}

// --- UpdateRecord round-trip ---

func TestCloudNS_CloudNSClient_UpdateRecord_ViaDoRaw_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "example.com", r.URL.Query().Get("domain-name"))
		assert.Equal(t, "42", r.URL.Query().Get("record-id"))
		assert.Equal(t, "www", r.URL.Query().Get("host"))
		assert.Equal(t, "A", r.URL.Query().Get("record-type"))
		assert.Equal(t, "5.6.7.8", r.URL.Query().Get("record"))
		assert.Equal(t, "3600", r.URL.Query().Get("ttl"))

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
	require.NoError(t, err)
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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Record not found")
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
	require.NoError(t, err)
	assert.False(t, changed, "should not change when record already correct")
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
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "5.6.7.8", r.URL.Query().Get("record"))
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
	require.NoError(t, err)
	assert.True(t, changed, "should change when record needs update")
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
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "www", r.URL.Query().Get("host"))
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
	require.NoError(t, err)
	assert.True(t, changed, "should change when record needs to be created")
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
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "2", r.URL.Query().Get("record-id"))
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
	require.NoError(t, err)
	assert.GreaterOrEqual(t, callCount, 2, "should have called list + delete")
}

func TestCloudNS_CloudNSClient_DoRaw_AuthQueryParams_Good(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "49500", r.URL.Query().Get("auth-id"))
		assert.Equal(t, "supersecret", r.URL.Query().Get("auth-password"))

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
	require.NoError(t, err)

	_, err = client.doRaw(req)
	require.NoError(t, err)
}
