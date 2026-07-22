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

| Method                                                        | Description                                                |
| ------------------------------------------------------------- | ---------------------------------------------------------- |
| `Insert(ctx, tableId, *T) error`                              | Insert a single row.                                       |
| `BulkInsert(ctx, tableId, []T) error`                         | Insert multiple rows in one request.                       |
| `Find(ctx, tableId, query, ...FindOptions) ([]T, error)`      | Search rows matching the filter map.                       |
| `Update(ctx, tableId, query, *T) ([]T, error)`                | Update rows matching the filter; returns the updated rows. |
| `Delete(ctx, tableId, query) error`                           | Delete rows matching the filter.                           |

The `query` map is sent to cnips as `{"filters": <query>}` and follows the cnips row-search semantics.

### Pagination (FindOptions)

`Find` accepts an optional `FindOptions` struct to control pagination:

```go
type FindOptions struct {
    Size int // Number of rows to return (default: 1000)
    Page int // Zero-based page index (default: 0)
}
```

When no `FindOptions` is provided, `Find` defaults to `size=1000`. This avoids the server-side default of 10 rows while keeping a reasonable limit. Use `FindOptions` to paginate or increase the limit for larger datasets.

```go
// Get all rows (uses default size=10000)
users, err := accessor.Find(ctx, tableID, map[string]any{})

// Paginate explicitly
page1, err := accessor.Find(ctx, tableID, map[string]any{}, table.FindOptions{Size: 50, Page: 0})
page2, err := accessor.Find(ctx, tableID, map[string]any{}, table.FindOptions{Size: 50, Page: 1})
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
