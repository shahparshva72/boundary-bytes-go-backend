package health

import "testing"

type fakeRepository struct {
	status map[string]string
}

func (r fakeRepository) Health() map[string]string {
	return r.status
}

func TestDBHealthReturnsRepositoryStatus(t *testing.T) {
	service := New(fakeRepository{status: map[string]string{"status": "up"}})

	got := service.DBHealth()
	if got["status"] != "up" {
		t.Fatalf("status = %q, want up", got["status"])
	}
}
