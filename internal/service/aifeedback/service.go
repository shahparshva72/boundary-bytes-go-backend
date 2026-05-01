package aifeedback

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

var (
	ErrRequestIDRequired       = errors.New("Request ID is required")
	ErrAccuracyRequired        = errors.New("isAccurate is required")
	ErrFeedbackNoteTooLong     = errors.New("Feedback note is too long (max 1000 characters)")
	ErrRequestNotFound         = errors.New("Request not found")
	ErrFeedbackAlreadyProvided = errors.New("Feedback has already been submitted for this request")
)

type Repository interface {
	GetAIAccuracyStats(ctx context.Context) (models.AIFeedbackStats, error)
	GetAIRequestByID(ctx context.Context, id string) (*models.AIChatRequestRecord, error)
	MarkAIRequestAccuracy(ctx context.Context, requestID string, isAccurate bool, feedbackNote *string) error
}

type Service struct {
	repository Repository
}

type SubmitParams struct {
	RequestID    string
	IsAccurate   *bool
	FeedbackNote *string
}

func New(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Submit(ctx context.Context, params SubmitParams) error {
	requestID := strings.TrimSpace(params.RequestID)
	if requestID == "" {
		return ErrRequestIDRequired
	}
	if params.IsAccurate == nil {
		return ErrAccuracyRequired
	}

	feedbackNote, err := cleanFeedbackNote(params.FeedbackNote)
	if err != nil {
		return err
	}

	existingRequest, err := s.repository.GetAIRequestByID(ctx, requestID)
	if err != nil {
		return err
	}
	if existingRequest == nil {
		return ErrRequestNotFound
	}
	if existingRequest.IsAccurate != nil {
		return ErrFeedbackAlreadyProvided
	}

	if err := s.repository.MarkAIRequestAccuracy(ctx, requestID, *params.IsAccurate, feedbackNote); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRequestNotFound
		}
		return err
	}

	return nil
}

func (s *Service) Stats(ctx context.Context) (models.AIFeedbackStats, error) {
	return s.repository.GetAIAccuracyStats(ctx)
}

func cleanFeedbackNote(note *string) (*string, error) {
	if note == nil {
		return nil, nil
	}

	trimmed := strings.TrimSpace(*note)
	if len(trimmed) > 1000 {
		return nil, ErrFeedbackNoteTooLong
	}
	if trimmed == "" {
		return nil, nil
	}
	return &trimmed, nil
}
