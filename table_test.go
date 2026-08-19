package table

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type TestData struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// UnmarshalableData is a test type that cannot be marshaled
type UnmarshalableData struct {
	ID int `json:"id"`
}

func (u UnmarshalableData) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("marshal error")
}

func TestNewCnipsTableAccessor(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")

		if accessor == nil {
			t.Fatal("NewCnipsTableAccessor returned nil")
		}

		if accessor.BaseURL != "https://example.com" {
			t.Errorf("expected BaseURL 'https://example.com', got '%s'", accessor.BaseURL)
		}

		if accessor.ApiKey != "api-key" {
			t.Errorf("expected ApiKey 'api-key', got '%s'", accessor.ApiKey)
		}

		if accessor.client == nil {
			t.Fatal("client is nil")
		}

		if accessor.client.Timeout != 60*time.Second {
			t.Errorf("expected default timeout 60s, got %v", accessor.client.Timeout)
		}
	})
}

func TestNewCnipsTableAccessorWithConfig(t *testing.T) {
	t.Run("WithCustomClient", func(t *testing.T) {
		customClient := &http.Client{Timeout: 10 * time.Second}
		accessor := NewCnipsTableAccessorWithConfig[TestData](
			"https://example.com",
			"api-key",
			Config{HTTPClient: customClient},
		)

		if accessor.client != customClient {
			t.Error("expected custom client to be used")
		}
	})
}

func TestBuildURL(t *testing.T) {
	accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
	t.Run("Basic", func(t *testing.T) {
		url, err := accessor.buildURL("table1")
		if err != nil {
			t.Fatalf("buildURL failed: %v", err)
		}
		if url != "https://example.com/tables/table1/rows" {
			t.Errorf("expected URL 'https://example.com/tables/table1/rows', got '%s'", url)
		}
	})

	t.Run("EmptyTableId", func(t *testing.T) {
		_, err := accessor.buildURL("")
		if err == nil {
			t.Fatal("expected error for empty tableId")
		}
	})

	t.Run("InvalidBaseURL", func(t *testing.T) {
		accessor.BaseURL = "://invalid"
		_, err := accessor.buildURL("table1")
		if err == nil {
			t.Fatal("expected error for invalid base URL")
		}
	})
}

func TestBuildSearchURL(t *testing.T) {
	accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
	t.Run("Basic", func(t *testing.T) {
		url, err := accessor.buildSearchURL("table1")
		if err != nil {
			t.Fatalf("buildSearchURL failed: %v", err)
		}
		if url != "https://example.com/tables/table1/rows/search" {
			t.Errorf("expected URL 'https://example.com/tables/table1/rows/search', got '%s'", url)
		}
	})

	t.Run("EmptyTableId", func(t *testing.T) {
		_, err := accessor.buildSearchURL("")
		if err == nil {
			t.Fatal("expected error for empty tableId")
		}
	})

	t.Run("InvalidBaseURL", func(t *testing.T) {
		accessor.BaseURL = "://invalid"
		_, err := accessor.buildSearchURL("table1")
		if err == nil {
			t.Fatal("expected error for invalid base URL")
		}
	})
}

func TestSetHeaders(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		request := &http.Request{
			Header: make(http.Header),
		}
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		accessor.setHeaders(request)
		if request.Header.Get("X-API-Key") != "api-key" {
			t.Error("missing or incorrect X-API-Key header")
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Error("missing or incorrect Content-Type header")
		}
	})
}

func TestDoRequest(t *testing.T) {
	accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
	t.Run("Basic", func(t *testing.T) {
		_, err := accessor.doRequest(context.Background(), http.MethodPost, "https://example.com", nil)
		if err != nil {
			t.Fatalf("doRequest failed: %v", err)
		}
	})

	t.Run("InvalidRequest", func(t *testing.T) {
		_, err := accessor.doRequest(context.Background(), "gg", "://invalid", nil)
		if err == nil {
			t.Fatal("expected error for invalid request")
		}
	})

	t.Run("ClientDoError_Timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		_, err := accessor.doRequest(ctx, http.MethodPost, "https://example.com", nil)
		if err == nil {
			t.Fatal("expected error for timeout")
		}
	})
}

func TestHandleResponse(t *testing.T) {
	accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
	t.Run("Success", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"message": "success"}`)),
		}
		err := accessor.handleResponse(resp)
		if err != nil {
			t.Fatalf("handleResponse failed: %v", err)
		}
	})
	t.Run("BodyReadError", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       &errorReaderCloser{},
		}
		err := accessor.handleResponse(resp)
		if err == nil {
			t.Fatalf("expected error for body read error")
		}
	})
	t.Run("ErrorBodyLength500", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(`{"message": "error"}` + strings.Repeat("a", 500))),
		}
		err := accessor.handleResponse(resp)
		if err == nil {
			t.Fatalf("expected error for body length error")
		}
	})
}

type errorReaderCloser struct{}

func (e *errorReaderCloser) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}

func (e *errorReaderCloser) Close() error {
	return nil
}

func TestInsert(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("X-API-Key") != "api-key" {
				t.Error("missing or incorrect X-API-Key header")
			}
			var data []TestData
			if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			if len(data) != 1 {
				t.Errorf("expected 1 item, got %d", len(data))
			}
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		data := &TestData{ID: 1, Name: "test"}
		err := accessor.Insert(context.Background(), "table1", data)
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	})

	t.Run("SuccessOK", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		data := &TestData{ID: 1, Name: "test"}
		err := accessor.Insert(context.Background(), "table1", data)
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	})

	t.Run("NilData", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		err := accessor.Insert(context.Background(), "table1", nil)
		if err == nil {
			t.Fatal("expected error for nil data")
		}
		if err.Error() != "data cannot be nil" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("EmptyTableId", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		data := &TestData{ID: 1, Name: "test"}
		err := accessor.Insert(context.Background(), "", data)
		if err == nil {
			t.Fatal("expected error for empty tableId")
		}
	})

	t.Run("InvalidBaseURL", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("://invalid", "api-key")
		data := &TestData{ID: 1, Name: "test"}
		err := accessor.Insert(context.Background(), "table1", data)
		if err == nil {
			t.Fatal("expected error for invalid base URL")
		}
	})

	t.Run("ErrorStatus", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("error message"))
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		data := &TestData{ID: 1, Name: "test"}
		err := accessor.Insert(context.Background(), "table1", data)
		if err == nil {
			t.Fatal("expected error for bad status")
		}
	})

	t.Run("MarshalError", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[UnmarshalableData]("https://example.com", "api-key")
		data := &UnmarshalableData{ID: 1}
		err := accessor.Insert(context.Background(), "table1", data)
		if err == nil {
			t.Fatal("expected error for unmarshalable data")
		}
	})

	t.Run("NetworkError", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("http://127.0.0.1:1", "api-key")
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		data := &TestData{ID: 1, Name: "test"}
		err := accessor.Insert(ctx, "table1", data)
		if err == nil {
			t.Fatal("expected error for network failure")
		}
	})
}

func TestBulkInsert(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			var data []TestData
			if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			if len(data) != 2 {
				t.Errorf("expected 2 items, got %d", len(data))
			}
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		data := []TestData{
			{ID: 1, Name: "test1"},
			{ID: 2, Name: "test2"},
		}
		err := accessor.BulkInsert(context.Background(), "table1", data)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}
	})

	t.Run("EmptyData", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		err := accessor.BulkInsert(context.Background(), "table1", []TestData{})
		if err == nil {
			t.Fatal("expected error for empty data")
		}
		if err.Error() != "data cannot be empty" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("ExceedsMaxRows", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		data := make([]TestData, MaxRowsPerRequest+1)
		for i := range data {
			data[i] = TestData{ID: i, Name: fmt.Sprintf("test%d", i)}
		}
		err := accessor.BulkInsert(context.Background(), "table1", data)
		if err == nil {
			t.Fatal("expected error for exceeding max rows")
		}
		if !strings.Contains(err.Error(), "limited to") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("EmptyTableId", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		data := []TestData{{ID: 1, Name: "test"}}
		err := accessor.BulkInsert(context.Background(), "", data)
		if err == nil {
			t.Fatal("expected error for empty tableId")
		}
	})

	t.Run("ErrorStatus", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		data := []TestData{{ID: 1, Name: "test"}}
		err := accessor.BulkInsert(context.Background(), "table1", data)
		if err == nil {
			t.Fatal("expected error for bad status")
		}
	})

	t.Run("MarshalError", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[UnmarshalableData]("https://example.com", "api-key")
		data := []UnmarshalableData{{ID: 1}}
		err := accessor.BulkInsert(context.Background(), "table1", data)
		if err == nil {
			t.Fatal("expected error for unmarshalable data")
		}
	})

	t.Run("NetworkError", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("http://127.0.0.1:1", "api-key")
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		data := []TestData{{ID: 1, Name: "test"}}
		err := accessor.BulkInsert(ctx, "table1", data)
		if err == nil {
			t.Fatal("expected error for network failure")
		}
	})
}

func TestFind(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			var requestBody map[string]any
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			response := map[string]any{
				"success": true,
				"data": map[string]any{
					"list": []map[string]any{
						{"data": map[string]any{"id": 1, "name": "test1"}},
						{"data": map[string]any{"id": 2, "name": "test2"}},
					},
					"count": 5,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		results, err := accessor.Find(context.Background(), "table1", map[string]any{"id": 1})
		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("WithPaginationOptions", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("size") != "50" {
				t.Errorf("expected size=50, got %s", r.URL.Query().Get("size"))
			}
			if r.URL.Query().Get("page") != "2" {
				t.Errorf("expected page=2, got %s", r.URL.Query().Get("page"))
			}
			response := map[string]any{
				"success": true,
				"data": map[string]any{
					"list": []map[string]any{
						{"data": map[string]any{"id": 1, "name": "test1"}},
					},
					"count": 120,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		results, err := accessor.Find(context.Background(), "table1", map[string]any{}, FindOptions{Size: 50, Page: 2})
		if err != nil {
			t.Fatalf("Find with options failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("EmptyTableId", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		_, err := accessor.Find(context.Background(), "", map[string]any{})
		if err == nil {
			t.Fatal("expected error for empty tableId")
		}
	})

	t.Run("ErrorStatus", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found " + strings.Repeat("a", 500)))
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		_, err := accessor.Find(context.Background(), "table1", map[string]any{})
		if err == nil {
			t.Fatal("expected error for bad status")
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		_, err := accessor.Find(context.Background(), "table1", map[string]any{})
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("InvalidDataJSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]any{
				"success": true,
				"data": map[string]any{
					"list": []map[string]any{
						{"data": map[string]any{"id": "not an int", "name": "test"}},
					},
					"count": 1,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		_, err := accessor.Find(context.Background(), "table1", map[string]any{})
		if err == nil {
			t.Fatal("expected error for invalid data type in response")
		}
	})

	t.Run("MarshalDataError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]any{
				"success": true,
				"data": map[string]any{
					"list": []map[string]any{
						{"data": UnmarshalableData{ID: 1}},
					},
					"count": 1,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		_, err := accessor.Find(context.Background(), "table1", map[string]any{})
		if err == nil {
			t.Fatal("expected error for unmarshalable data in response")
		}
	})

	t.Run("MarshalQueryError", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		query := map[string]any{
			"data": UnmarshalableData{ID: 1},
		}
		_, err := accessor.Find(context.Background(), "table1", query)
		if err == nil {
			t.Fatal("expected error for unmarshalable query")
		}
	})

	t.Run("NetworkError", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("http://127.0.0.1:1", "api-key")
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := accessor.Find(ctx, "table1", map[string]any{})
		if err == nil {
			t.Fatal("expected error for network failure")
		}
	})
}

func TestDelete(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("expected DELETE, got %s", r.Method)
			}
			var requestBody map[string]any
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		err := accessor.Delete(context.Background(), "table1", map[string]any{"id": 1})
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
	})

	t.Run("SuccessNoContent", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		err := accessor.Delete(context.Background(), "table1", map[string]any{"id": 1})
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
	})

	t.Run("EmptyTableId", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		err := accessor.Delete(context.Background(), "", map[string]any{})
		if err == nil {
			t.Fatal("expected error for empty tableId")
		}
	})

	t.Run("ErrorStatus", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("forbidden"))
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		err := accessor.Delete(context.Background(), "table1", map[string]any{})
		if err == nil {
			t.Fatal("expected error for bad status")
		}
	})

	t.Run("MarshalQueryError", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		query := map[string]any{
			"data": UnmarshalableData{ID: 1},
		}
		err := accessor.Delete(context.Background(), "table1", query)
		if err == nil {
			t.Fatal("expected error for unmarshalable query")
		}
	})

	t.Run("NetworkError", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("http://127.0.0.1:1", "api-key")
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := accessor.Delete(ctx, "table1", map[string]any{})
		if err == nil {
			t.Fatal("expected error for network failure")
		}
	})
}

func TestUpdate(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("expected PUT, got %s", r.Method)
			}
			var requestBody map[string]any
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			response := map[string]any{
				"success": true,
				"data": []map[string]any{
					{"data": map[string]any{"id": 1, "name": "updated"}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		data := &TestData{ID: 1, Name: "updated"}
		results, err := accessor.Update(context.Background(), "table1", map[string]any{"id": 1}, data)
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}
		if results[0].Name != "updated" {
			t.Errorf("expected name 'updated', got '%s'", results[0].Name)
		}
	})

	t.Run("NilData", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		_, err := accessor.Update(context.Background(), "table1", map[string]any{}, nil)
		if err == nil {
			t.Fatal("expected error for nil data")
		}
		if err.Error() != "data cannot be nil" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("EmptyTableId", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		data := &TestData{ID: 1, Name: "test"}
		_, err := accessor.Update(context.Background(), "", map[string]any{}, data)
		if err == nil {
			t.Fatal("expected error for empty tableId")
		}
	})

	t.Run("ErrorStatus", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("bad request"))
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		data := &TestData{ID: 1, Name: "test"}
		_, err := accessor.Update(context.Background(), "table1", map[string]any{}, data)
		if err == nil {
			t.Fatal("expected error for bad status")
		}
	})

	t.Run("ErrorStatusLongMessage", func(t *testing.T) {
		longMessage := make([]byte, 600)
		for i := range longMessage {
			longMessage[i] = 'b'
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write(longMessage)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		data := &TestData{ID: 1, Name: "test"}
		_, err := accessor.Update(context.Background(), "table1", map[string]any{}, data)
		if err == nil {
			t.Fatal("expected error for bad status")
		}
		if len(err.Error()) <= 500 {
			t.Error("expected error message to be truncated")
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		data := &TestData{ID: 1, Name: "test"}
		_, err := accessor.Update(context.Background(), "table1", map[string]any{}, data)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("InvalidDataJSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]any{
				"success": true,
				"data": []map[string]any{
					{"data": map[string]any{"id": "not an int", "name": "test"}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		data := &TestData{ID: 1, Name: "test"}
		_, err := accessor.Update(context.Background(), "table1", map[string]any{}, data)
		if err == nil {
			t.Fatal("expected error for invalid data type in response")
		}
	})

	t.Run("MarshalDataError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]any{
				"success": true,
				"data": []map[string]any{
					{"data": UnmarshalableData{ID: 1}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		data := &TestData{ID: 1, Name: "test"}
		_, err := accessor.Update(context.Background(), "table1", map[string]any{}, data)
		if err == nil {
			t.Fatal("expected error for unmarshalable data in response")
		}
	})

	t.Run("MarshalRequestBodyError", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		query := map[string]any{
			"data": UnmarshalableData{ID: 1},
		}
		data := &TestData{ID: 1, Name: "test"}
		_, err := accessor.Update(context.Background(), "table1", query, data)
		if err == nil {
			t.Fatal("expected error for unmarshalable query in request body")
		}
	})

	t.Run("MarshalDataInBodyError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]any{
				"success": true,
				"data": []map[string]any{
					{"data": UnmarshalableData{ID: 1}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		data := &TestData{ID: 1, Name: "test"}
		_, err := accessor.Update(context.Background(), "table1", map[string]any{}, data)
		if err == nil {
			t.Fatal("expected error for unmarshalable data in request body")
		}
	})

	t.Run("NetworkError", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("http://127.0.0.1:1", "api-key")
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := accessor.Update(ctx, "table1", map[string]any{}, &TestData{ID: 1, Name: "test"})
		if err == nil {
			t.Fatal("expected error for network failure")
		}
	})
}

func TestCount(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.URL.Query().Get("size") != "1" {
				t.Errorf("expected size=1, got %s", r.URL.Query().Get("size"))
			}
			response := map[string]any{
				"success": true,
				"data": map[string]any{
					"list":  []map[string]any{},
					"count": 42,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		count, err := accessor.Count(context.Background(), "table1", map[string]any{"department": "IT"})
		if err != nil {
			t.Fatalf("Count failed: %v", err)
		}
		if count != 42 {
			t.Errorf("expected count 42, got %d", count)
		}
	})

	t.Run("EmptyQuery", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]any{
				"success": true,
				"data": map[string]any{
					"list":  []map[string]any{},
					"count": 100,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		count, err := accessor.Count(context.Background(), "table1", map[string]any{})
		if err != nil {
			t.Fatalf("Count failed: %v", err)
		}
		if count != 100 {
			t.Errorf("expected count 100, got %d", count)
		}
	})

	t.Run("EmptyTableId", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		_, err := accessor.Count(context.Background(), "", map[string]any{})
		if err == nil {
			t.Fatal("expected error for empty tableId")
		}
	})

	t.Run("ErrorStatus", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		_, err := accessor.Count(context.Background(), "table1", map[string]any{})
		if err == nil {
			t.Fatal("expected error for bad status")
		}
	})

	t.Run("FailedResponse", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]any{
				"success": false,
				"data": map[string]any{
					"count": 0,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		_, err := accessor.Count(context.Background(), "table1", map[string]any{})
		if err == nil {
			t.Fatal("expected error for failed response")
		}
	})

	t.Run("NetworkError", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("http://127.0.0.1:1", "api-key")
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := accessor.Count(ctx, "table1", map[string]any{})
		if err == nil {
			t.Fatal("expected error for network failure")
		}
	})
}

func TestDeleteByIds(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("expected DELETE, got %s", r.Method)
			}
			var requestBody map[string]any
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			ids, ok := requestBody["rowIds"]
			if !ok {
				t.Fatal("expected rowIds in request body")
			}
			idsList, ok := ids.([]any)
			if !ok || len(idsList) != 2 {
				t.Errorf("expected 2 rowIds, got %v", ids)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		err := accessor.DeleteByIds(context.Background(), "table1", []string{"id-1", "id-2"})
		if err != nil {
			t.Fatalf("DeleteByIds failed: %v", err)
		}
	})

	t.Run("EmptyRowIds", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		err := accessor.DeleteByIds(context.Background(), "table1", []string{})
		if err == nil {
			t.Fatal("expected error for empty rowIds")
		}
		if err.Error() != "rowIds cannot be empty" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("ExceedsMaxIds", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		ids := make([]string, MaxDeleteIDs+1)
		for i := range ids {
			ids[i] = fmt.Sprintf("id-%d", i)
		}
		err := accessor.DeleteByIds(context.Background(), "table1", ids)
		if err == nil {
			t.Fatal("expected error for exceeding max IDs")
		}
		if !strings.Contains(err.Error(), "limited to") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("EmptyTableId", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("https://example.com", "api-key")
		err := accessor.DeleteByIds(context.Background(), "", []string{"id-1"})
		if err == nil {
			t.Fatal("expected error for empty tableId")
		}
	})

	t.Run("ErrorStatus", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("forbidden"))
		}))
		defer server.Close()

		accessor := NewCnipsTableAccessor[TestData](server.URL, "api-key")
		err := accessor.DeleteByIds(context.Background(), "table1", []string{"id-1"})
		if err == nil {
			t.Fatal("expected error for bad status")
		}
	})

	t.Run("NetworkError", func(t *testing.T) {
		accessor := NewCnipsTableAccessor[TestData]("http://127.0.0.1:1", "api-key")
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := accessor.DeleteByIds(ctx, "table1", []string{"id-1"})
		if err == nil {
			t.Fatal("expected error for network failure")
		}
	})
}
