package leagues

import (
	"context"
	"testing"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

type fakeRepository struct {
	configs []models.LeagueConfigItem
}

func (r fakeRepository) GetLeagueConfigStats(ctx context.Context) ([]models.LeagueConfigItem, error) {
	return r.configs, nil
}

func TestGetConfigsReturnsEmptySliceForNilRepositoryResult(t *testing.T) {
	service := New(fakeRepository{})

	got, err := service.GetConfigs(context.Background())
	if err != nil {
		t.Fatalf("GetConfigs() error = %v", err)
	}
	if got == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("len(configs) = %d, want 0", len(got))
	}
}
