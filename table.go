package table

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	MaxRowsPerRequest = 20
	MaxDeleteIDs      = 100
)

// FindOptions controls pagination for Find queries.
// The server defaults to page=0 and size=10. Maximum size is 20.
type FindOptions struct {
	// Size is the number of rows per page (1–20). If 0, the server uses its default of 10.
	Size int
	// Page is the zero-based page index. If 0, returns the first page.
	Page int
}

// TableAccessor defines the interface for accessing table data.
type TableAccessor[T any] interface {
	Insert(ctx context.Context, tableId string, data *T) error
	BulkInsert(ctx context.Context, tableId string, data []T) error
	Find(ctx context.Context, tableId string, query map[string]any, opts ...FindOptions) ([]T, error)
	Count(ctx context.Context, tableId string, query map[string]any) (int, error)
	Delete(ctx context.Context, tableId string, query map[string]any) error
	DeleteByIds(ctx context.Context, tableId string, rowIds []string) error
	Update(ctx context.Context, tableId string, query map[string]any, data *T) ([]T, error)
}

// CnipsTableAccessor is a concrete implementation of TableAccessor for CNIPS.
type CnipsTableAccessor[T any] struct {
	client  *http.Client
	BaseURL string
	ApiKey  string
}

// Config holds configuration options for CnipsTableAccessor.
type Config struct {
	// Timeout specifies the timeout for HTTP requests.
	// If zero, defaults to 30 seconds.
	Timeout time.Duration
	// HTTPClient allows providing a custom HTTP client.
	// If nil, a default client with timeout will be created.
	HTTPClient *http.Client
}

// NewCnipsTableAccessor creates a new CnipsTableAccessor instance.
// The parameter baseURL is the base URL of the CNIPS instance and it is mandatory.
// The parameter apiKey is the API key that can be created in the CNIPS instance and it is mandatory.
func NewCnipsTableAccessor[T any](baseURL, apiKey string) *CnipsTableAccessor[T] {
	return NewCnipsTableAccessorWithConfig[T](baseURL, apiKey, Config{})
}

// NewCnipsTableAccessorWithConfig creates a new CnipsTableAccessor instance with custom configuration.
func NewCnipsTableAccessorWithConfig[T any](baseURL, apiKey string, config Config) *CnipsTableAccessor[T] {
	client := config.HTTPClient
	if client == nil {
		timeout := config.Timeout
		if timeout == 0 {
			timeout = 60 * time.Second
		}
		client = &http.Client{
			Timeout: timeout,
		}
	}

	return &CnipsTableAccessor[T]{
		client:  client,
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		ApiKey:  apiKey,
	}
}

// buildURL builds the URL for the request.
func (t *CnipsTableAccessor[T]) buildURL(tableId string) (string, error) {
	if tableId == "" {
		return "", fmt.Errorf("tableId cannot be empty")
	}

	base, err := url.Parse(t.BaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	path := fmt.Sprintf("/tables/%s/rows", url.PathEscape(tableId))
	base.Path = strings.TrimSuffix(base.Path, "/") + path

	return base.String(), nil
}

// buildSearchURL builds the URL for the search endpoint.
func (t *CnipsTableAccessor[T]) buildSearchURL(tableId string) (string, error) {
	if tableId == "" {
		return "", fmt.Errorf("tableId cannot be empty")
	}

	base, err := url.Parse(t.BaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	path := fmt.Sprintf("/tables/%s/rows/search", url.PathEscape(tableId))
	base.Path = strings.TrimSuffix(base.Path, "/") + path

	return base.String(), nil
}

// setHeaders sets the headers for the request.
func (t *CnipsTableAccessor[T]) setHeaders(req *http.Request) {
	req.Header.Set("X-API-Key", t.ApiKey)
	req.Header.Set("Content-Type", "application/json")
}

// doRequest performs an HTTP request and handles the response.
func (t *CnipsTableAccessor[T]) doRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	t.setHeaders(req)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// handleResponse reads the response body and checks the status code.
func (t *CnipsTableAccessor[T]) handleResponse(resp *http.Response, expectedStatusCodes ...int) error {
	defer resp.Body.Close()

	// Read response body for error messages
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check if status code is expected
	if len(expectedStatusCodes) == 0 {
		expectedStatusCodes = []int{http.StatusOK, http.StatusCreated}
	}

	statusOK := slices.Contains(expectedStatusCodes, resp.StatusCode)

	if !statusOK {
		bodyStr := string(bodyBytes)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		return fmt.Errorf("unexpected status code %d: %s (response: %s)", resp.StatusCode, resp.Status, bodyStr)
	}

	return nil
}

// Insert inserts a new record into the specified table.
// The data is wrapped in an array as the server expects []map[string]interface{}.
func (t *CnipsTableAccessor[T]) Insert(ctx context.Context, tableId string, data *T) error {
	if data == nil {
		return fmt.Errorf("data cannot be nil")
	}

	requestURL, err := t.buildURL(tableId)
	if err != nil {
		return err
	}

	// Wrap single object in array as the server expects []map[string]interface{}
	body, err := json.Marshal([]*T{data})
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	resp, err := t.doRequest(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	return t.handleResponse(resp, http.StatusCreated, http.StatusOK)
}

// BulkInsert inserts multiple records into the specified table.
// Maximum 20 rows per request.
func (t *CnipsTableAccessor[T]) BulkInsert(ctx context.Context, tableId string, data []T) error {
	if len(data) == 0 {
		return fmt.Errorf("data cannot be empty")
	}
	if len(data) > MaxRowsPerRequest {
		return fmt.Errorf("bulk insert limited to %d rows per request, got %d", MaxRowsPerRequest, len(data))
	}

	requestURL, err := t.buildURL(tableId)
	if err != nil {
		return err
	}

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	resp, err := t.doRequest(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	return t.handleResponse(resp, http.StatusCreated, http.StatusOK)
}

// Find retrieves records from the specified table matching the query.
// Uses POST request to /search endpoint with query in the request body.
// Without FindOptions the server applies its defaults (page=0, size=10).
// Pass FindOptions to control pagination. Maximum size is 20.
func (t *CnipsTableAccessor[T]) Find(ctx context.Context, tableId string, query map[string]any, opts ...FindOptions) ([]T, error) {
	requestURL, err := t.buildSearchURL(tableId)
	if err != nil {
		return nil, err
	}

	if len(opts) > 0 {
		parsed, err := url.Parse(requestURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse search URL: %w", err)
		}
		q := parsed.Query()
		if opts[0].Size > 0 {
			q.Set("size", strconv.Itoa(opts[0].Size))
		}
		if opts[0].Page > 0 {
			q.Set("page", strconv.Itoa(opts[0].Page))
		}
		parsed.RawQuery = q.Encode()
		requestURL = parsed.String()
	}

	requestBody := map[string]any{
		"filters": query,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	resp, err := t.doRequest(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		return nil, fmt.Errorf("unexpected status code %d: %s (response: %s)", resp.StatusCode, resp.Status, bodyStr)
	}

	var results struct {
		Success bool `json:"success"`
		Data    struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !results.Success {
		return nil, fmt.Errorf("failed to get data")
	}

	resultsT := make([]T, len(results.Data.List))
	for i, result := range results.Data.List {
		dataBytes, err := json.Marshal(result["data"])
		if err != nil {
			return nil, fmt.Errorf("failed to marshal row data: %w", err)
		}
		var item T
		if err := json.Unmarshal(dataBytes, &item); err != nil {
			return nil, fmt.Errorf("failed to unmarshal row data: %w", err)
		}
		resultsT[i] = item
	}

	return resultsT, nil
}

// Count returns the total number of rows matching the query.
// Uses the same /search endpoint with size=1 to minimize payload.
func (t *CnipsTableAccessor[T]) Count(ctx context.Context, tableId string, query map[string]any) (int, error) {
	requestURL, err := t.buildSearchURL(tableId)
	if err != nil {
		return 0, err
	}

	parsed, err := url.Parse(requestURL)
	if err != nil {
		return 0, fmt.Errorf("failed to parse search URL: %w", err)
	}
	q := parsed.Query()
	q.Set("size", "1")
	parsed.RawQuery = q.Encode()
	requestURL = parsed.String()

	requestBody := map[string]any{
		"filters": query,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal query: %w", err)
	}

	resp, err := t.doRequest(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		return 0, fmt.Errorf("unexpected status code %d: %s (response: %s)", resp.StatusCode, resp.Status, bodyStr)
	}

	var results struct {
		Success bool `json:"success"`
		Data    struct {
			Count int `json:"count"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if !results.Success {
		return 0, fmt.Errorf("failed to get count")
	}

	return results.Data.Count, nil
}

// Delete removes records from the specified table matching the query.
// Uses DELETE request with query in the request body.
func (t *CnipsTableAccessor[T]) Delete(ctx context.Context, tableId string, query map[string]any) error {
	requestURL, err := t.buildURL(tableId)
	if err != nil {
		return err
	}

	// Create request body with query
	requestBody := map[string]any{
		"filters": query,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal query: %w", err)
	}

	resp, err := t.doRequest(ctx, http.MethodDelete, requestURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	return t.handleResponse(resp, http.StatusOK, http.StatusNoContent)
}

// DeleteByIds removes specific rows by their IDs.
// Maximum 100 row IDs per request.
func (t *CnipsTableAccessor[T]) DeleteByIds(ctx context.Context, tableId string, rowIds []string) error {
	if len(rowIds) == 0 {
		return fmt.Errorf("rowIds cannot be empty")
	}
	if len(rowIds) > MaxDeleteIDs {
		return fmt.Errorf("deleteByIds limited to %d IDs per request, got %d", MaxDeleteIDs, len(rowIds))
	}

	requestURL, err := t.buildURL(tableId)
	if err != nil {
		return err
	}

	requestBody := map[string]any{
		"rowIds": rowIds,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal rowIds: %w", err)
	}

	resp, err := t.doRequest(ctx, http.MethodDelete, requestURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	return t.handleResponse(resp, http.StatusOK, http.StatusNoContent)
}

// Update updates records in the specified table matching the query with the provided data.
// Uses PUT request with query and data in the request body.
func (t *CnipsTableAccessor[T]) Update(ctx context.Context, tableId string, query map[string]any, data *T) ([]T, error) {
	if data == nil {
		return nil, fmt.Errorf("data cannot be nil")
	}

	requestURL, err := t.buildURL(tableId)
	if err != nil {
		return nil, err
	}

	// Create request body with query and data
	requestBody := map[string]any{
		"filters": query,
		"data":    data,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	resp, err := t.doRequest(ctx, http.MethodPut, requestURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		return nil, fmt.Errorf("unexpected status code %d: %s (response: %s)", resp.StatusCode, resp.Status, bodyStr)
	}

	var results struct {
		Success bool             `json:"success"`
		Data    []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !results.Success {
		return nil, fmt.Errorf("failed to update data")
	}

	resultsT := make([]T, len(results.Data))
	for i, result := range results.Data {
		dataBytes, err := json.Marshal(result["data"])
		if err != nil {
			return nil, fmt.Errorf("failed to marshal row data: %w", err)
		}
		var item T
		if err := json.Unmarshal(dataBytes, &item); err != nil {
			return nil, fmt.Errorf("failed to unmarshal row data: %w", err)
		}
		resultsT[i] = item
	}

	return resultsT, nil
}
