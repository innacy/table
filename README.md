# table

A small, generic Go client for accessing tables in a [cnips](https://cnips.io) instance over HTTP.

The package exposes a typed `TableAccessor[T]` interface and a concrete `CnipsTableAccessor[T]` implementation that maps Go structs to rows in a cnips table.

## Installation

```bash
go get github.com/innacy/table
```

Requires Go 1.24+.

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/innacy/table"
)

type User struct {
    Name  string `json:"name"`
    Email string `json:"email"`
    Age   int    `json:"age"`
}

func main() {
    ctx := context.Background()
    accessor := table.NewCnipsTableAccessor[User](
        "https://your-cnips-instance.com",
        "your-api-key",
    )
    tableID := "users-table-id"

    // Insert a single row
    if err := accessor.Insert(ctx, tableID, &User{
        Name: "Ada", Email: "ada@example.com", Age: 36,
    }); err != nil {
        log.Fatal(err)
    }

    // Bulk insert
    if err := accessor.BulkInsert(ctx, tableID, []User{
        {Name: "Alan", Email: "alan@example.com", Age: 41},
        {Name: "Grace", Email: "grace@example.com", Age: 85},
    }); err != nil {
        log.Fatal(err)
    }

    // Find rows matching a query
    users, err := accessor.Find(ctx, tableID, map[string]any{
        "name": "Ada",
    })
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("found %d users", len(users))

    // Get total count of rows matching a query
    total, err := accessor.Count(ctx, tableID, map[string]any{})
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("table has %d total rows", total)

    // Update rows matching a query
    updated, err := accessor.Update(ctx, tableID, map[string]any{
        "name": "Ada",
    }, &User{Name: "Ada", Email: "ada@new.example.com", Age: 37})
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("updated %d rows", len(updated))

    // Delete rows matching a query
    if err := accessor.Delete(ctx, tableID, map[string]any{
        "name": "Ada",
    }); err != nil {
        log.Fatal(err)
    }
}
```

## Configuration

By default, requests use an `http.Client` with a 60s timeout. To customize:

```go
accessor := table.NewCnipsTableAccessorWithConfig[User](
    "https://your-cnips-instance.com",
    "your-api-key",
    table.Config{
        Timeout: 10 * time.Second,
        // Or supply your own client (Timeout is ignored if HTTPClient is set):
        // HTTPClient: myClient,
    },
)
```

## API

`TableAccessor[T any]` defines the operations supported by the package:

| Method                                                             | Description                                                |
| ------------------------------------------------------------------ | ---------------------------------------------------------- |
| `Insert(ctx, tableId, *T) error`                                   | Insert a single row.                                       |
| `BulkInsert(ctx, tableId, []T) error`                              | Insert multiple rows in one request.                       |
| `Find(ctx, tableId, query, ...FindOptions) ([]T, error)`           | Search rows matching the filter map.                       |
| `Count(ctx, tableId, query) (int, error)`                          | Get total number of rows matching the filter.              |
| `Update(ctx, tableId, query, *T) ([]T, error)`                     | Update rows matching the filter; returns the updated rows. |
| `Delete(ctx, tableId, query) error`                                | Delete rows matching the filter.                           |

The `query` map is sent to cnips as `{"filters": <query>}` and follows the cnips row-search semantics.

### Pagination (FindOptions)

You can pass an optional `FindOptions` struct to control pagination:

```go
type FindOptions struct {
    Size int // Rows per page (1–10000). Server default: 10.
    Page int // Zero-based page index. Server default: 0.
}
```

> **Note:** Without `FindOptions`, the server returns at most **10 rows** (the default page size). The maximum allowed size is **10000**.

Use `Count` to get the total number of matching rows, then paginate with `Find`:

```go
// Get total count
total, _ := accessor.Count(ctx, tableID, map[string]any{"department": "IT"})
fmt.Printf("%d total rows\n", total)

// Paginate through all results
pageSize := 50
for page := 0; page * pageSize < total; page++ {
    rows, _ := accessor.Find(ctx, tableID, map[string]any{"department": "IT"}, table.FindOptions{Size: pageSize, Page: page})
    // process rows...
}

// Or fetch up to 10000 rows at once
all, _ := accessor.Find(ctx, tableID, map[string]any{}, table.FindOptions{Size: 10000})
```

### HTTP endpoints used

- `POST /tables/{tableId}/rows` — insert / bulk insert
- `POST /tables/{tableId}/rows/search?size=N&page=N` — find
- `PUT  /tables/{tableId}/rows` — update
- `DELETE /tables/{tableId}/rows` — delete

Authentication uses the `X-API-Key` header.

## Testing

```bash
make test
```

This runs the test suite with coverage and writes `coverage.html`.

## License

See repository for license information.
