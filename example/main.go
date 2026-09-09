package main

import (
	"context"
	"log"
	"os"

	"github.com/innacy/table"
)

type Row struct {
	Name       string `json:"name,omitempty"`
	Department string `json:"department,omitempty"`
}

func main() {
	tableId := os.Getenv("tableId")
	tableBaseUrl := os.Getenv("table_base_url")
	apiKey := os.Getenv("apiKey")

	tableAccessor := table.NewCnipsTableAccessor[Row](tableBaseUrl, apiKey)

	ctx := context.Background()
	log.Printf("=== CNIPS Table Accessor Examples ===")

	// Example 1: Insert a single row
	exampleInsert(ctx, tableAccessor, tableId)

	// Example 2: Bulk insert multiple rows
	exampleBulkInsert(ctx, tableAccessor, tableId)

	// Example 3: Find rows with query
	exampleFind(ctx, tableAccessor, tableId)

	// Example 4: Update rows
	exampleUpdate(ctx, tableAccessor, tableId)

	// Example 5: Delete rows
	exampleDelete(ctx, tableAccessor, tableId)

	// Example 6: Count rows
	exampleCount(ctx, tableAccessor, tableId)

	log.Printf("task completed")
}

// exampleInsert demonstrates inserting a single record
func exampleInsert(ctx context.Context, tableAccessor *table.CnipsTableAccessor[Row], tableId string) {
	log.Printf("1. Insert Single Row")

	row := &Row{
		Name:       "John1 Doe1",
		Department: "IT",
	}

	err := tableAccessor.Insert(ctx, tableId, row)
	if err != nil {
		log.Fatalf("Error inserting row: %v", err)
		return
	}
	log.Printf("✓ Successfully inserted: %+v", row)
	log.Printf("----------------------------")
}

// exampleBulkInsert demonstrates inserting multiple records at once
func exampleBulkInsert(ctx context.Context, tableAccessor *table.CnipsTableAccessor[Row], tableId string) {
	log.Printf("2. Bulk Insert Multiple Rows")

	rows := []Row{
		{Name: "Jane Doe", Department: "HR"},
		{Name: "Jim Beam", Department: "Sales"},
		{Name: "Alice Smith", Department: "IT"},
		{Name: "Bob Johnson", Department: "Marketing"},
	}

	err := tableAccessor.BulkInsert(ctx, tableId, rows)
	if err != nil {
		log.Fatalf("Error bulk inserting rows: %v", err)
		return
	}
	log.Printf("✓ Successfully bulk inserted %d rows", len(rows))
	log.Printf("----------------------------")

}

// exampleFind demonstrates querying records with different query patterns
func exampleFind(ctx context.Context, tableAccessor *table.CnipsTableAccessor[Row], tableId string) {
	log.Printf("3. Find Rows with Query")

	// // Find all rows (empty query)
	log.Printf("Finding all rows...")
	allRows, err := tableAccessor.Find(ctx, tableId, nil, table.FindOptions{
		Size: 5,
	})
	if err != nil {
		log.Fatalf("Error finding rows: %v", err)
	} else {
		log.Printf("✓ Found %d rows", len(allRows))
		for i, row := range allRows {
			log.Printf("  [%d] %+v", i+1, row)
		}
	}

	// Find rows with specific department
	log.Printf("Finding rows in IT department...")
	query := map[string]any{
		"department": "IT",
	}
	itRows, err := tableAccessor.Find(ctx, tableId, query)
	if err != nil {
		log.Fatalf("Error finding rows: %v", err)
	} else {
		log.Printf("✓ Found %d IT employees", len(itRows))
		for i, row := range itRows {
			log.Printf("  [%d] %+v", i+1, row)
		}
	}
	log.Printf("------------------------")
}

// exampleCount demonstrates counting records with different query patterns
func exampleCount(ctx context.Context, tableAccessor *table.CnipsTableAccessor[Row], tableId string) {
	log.Printf("6. Count Rows with Query")

	// Count all rows (empty query)
	log.Printf("Counting all rows...")
	count, err := tableAccessor.Count(ctx, tableId, nil)
	if err != nil {
		log.Fatalf("Error counting rows: %v", err)
	} else {
		log.Printf("✓ Found %d rows", count)
	}

	// Count rows with specific department
	log.Printf("Counting rows in IT department...")
	query := map[string]any{
		"department": "IT",
	}
	queryCount, err := tableAccessor.Count(ctx, tableId, query)
	if err != nil {
		log.Fatalf("Error counting rows: %v", err)
	} else {
		log.Printf("✓ Found %d IT employees", queryCount)
	}
	log.Printf("------------------------")
}

// exampleUpdate demonstrates updating records
func exampleUpdate(ctx context.Context, tableAccessor *table.CnipsTableAccessor[Row], tableId string) {
	log.Printf("4. Update Rows")

	// Update all IT department employees
	query := map[string]any{
		"department": "IT",
	}
	updateData := &Row{
		Department: "Engineering", // Update department name
	}

	updatedRows, err := tableAccessor.Update(ctx, tableId, query, updateData)
	if err != nil {
		log.Fatalf("Error updating rows: %v", err)
		return
	}
	log.Printf("✓ Successfully updated %d rows", len(updatedRows))
	for i, row := range updatedRows {
		log.Printf("  [%d] %+v", i+1, row)
	}
	log.Printf("--------------")
}

// exampleDelete demonstrates deleting records
func exampleDelete(ctx context.Context, tableAccessor *table.CnipsTableAccessor[Row], tableId string) {
	log.Printf("5. Delete Rows")

	// Delete rows matching a specific condition
	query := map[string]any{
		"department": "Marketing",
	}

	err := tableAccessor.Delete(ctx, tableId, query)
	if err != nil {
		log.Fatalf("Error deleting rows: %v", err)
		return
	}
	log.Printf("✓ Successfully deleted rows matching query: %v", query)
	log.Printf("---------------")
}
