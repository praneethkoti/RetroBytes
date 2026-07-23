package services_test

import (
	"fmt"
	"testing"

	"retrobytes/internal/repos"
	"retrobytes/internal/services"
)

// SR-RATE-02: CatalogService.Search caps the number of rows returned no matter
// how large a pageSize a caller requests (CWE-770 defense-in-depth). Seed more
// products than the cap so an unbounded request would otherwise return them all.
func TestSearchPageSizeIsCapped(t *testing.T) {
	db, err := repos.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Insert 60 matching products (> maxPageSize of 50). All share the keyword
	// "widget" so a single search matches every one of them.
	for i := 0; i < 60; i++ {
		id := fmt.Sprintf("widget-%03d", i)
		if _, err := db.Exec(
			`INSERT INTO products(id,category_id,title,description,condition,price,images_json,active,created_at)
			 VALUES(?,?,?,?,?,?,?,1,CURRENT_TIMESTAMP)`,
			id, "retro-consoles", fmt.Sprintf("Widget %d", i), "test widget",
			"SECOND_HAND", 9.99, "[]"); err != nil {
			t.Fatalf("seed product %s: %v", id, err)
		}
	}

	catRepo := repos.NewCategoryRepo(db)
	prodRepo := repos.NewProductRepo(db)
	svc := services.NewCatalogService(catRepo, prodRepo)

	// Request an absurd page size; the service must clamp it.
	results, err := svc.Search("widget", "", "", 1, 100000)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) > 50 {
		t.Fatalf("search returned %d rows; expected at most the 50-row cap (CWE-770)", len(results))
	}
	if len(results) != 50 {
		t.Fatalf("expected exactly 50 rows (cap) given 60 matches, got %d", len(results))
	}
}
