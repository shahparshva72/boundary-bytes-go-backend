package stats

import "testing"

func TestTotalPages(t *testing.T) {
	if got := totalPages(21, 10); got != 3 {
		t.Fatalf("totalPages() = %d, want 3", got)
	}
}

func TestTotalPagesWithInvalidLimit(t *testing.T) {
	if got := totalPages(21, 0); got != 0 {
		t.Fatalf("totalPages() = %d, want 0", got)
	}
}
