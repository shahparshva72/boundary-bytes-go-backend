package health

type Repository interface {
	Health() map[string]string
}

type Service struct {
	repository Repository
}

func New(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) DBHealth() map[string]string {
	return s.repository.Health()
}
