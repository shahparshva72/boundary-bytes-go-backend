package leagues

import (
	"context"
	"testing"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/leagues"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

type fakeRepository struct {
	configs []models.LeagueConfigItem
}

func (r fakeRepository) GetLeagueConfigStats(ctx context.Context) ([]models.LeagueConfigItem, error) {
	return r.configs, nil
}

func testRegistry(t *testing.T) *leagues.Registry {
	t.Helper()
	r, err := leagues.Parse([]byte(`{"leagues":[
		{"code":"IPL","name":"Indian Premier League","gender":"male","visible":true},
		{"code":"WPL","name":"Women's Premier League","gender":"female","visible":true},
		{"code":"PSL","name":"Pakistan Super League","gender":"male","visible":false}
	]}`), false)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestGetConfigsListsVisibleLeaguesWithStats(t *testing.T) {
	service := New(fakeRepository{configs: []models.LeagueConfigItem{
		{League: "IPL", Stats: models.LeagueConfigStats{Teams: 10, Matches: 1243, Seasons: []string{"2008", "2026"}}},
		{League: "PSL", Stats: models.LeagueConfigStats{Matches: 357}},
	}}, testRegistry(t))

	got, err := service.GetConfigs(context.Background())
	if err != nil {
		t.Fatalf("GetConfigs() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(configs) = %d, want 2 (hidden PSL excluded): %+v", len(got), got)
	}

	if got[0].League != "IPL" || got[0].Name != "Indian Premier League" || got[0].Gender != "male" || got[0].Stats.Matches != 1243 {
		t.Errorf("IPL config = %+v", got[0])
	}
	if got[1].League != "WPL" || got[1].Gender != "female" || got[1].Stats.Matches != 0 || got[1].Stats.Seasons == nil {
		t.Errorf("WPL with no data should have zero stats and empty seasons: %+v", got[1])
	}
}
