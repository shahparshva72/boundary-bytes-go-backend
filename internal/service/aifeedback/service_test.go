package aifeedback

import (
	"context"
	"errors"
	"testing"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

type fakeRepository struct {
	request *models.AIChatRequestRecord
	marked  bool
}

func (r *fakeRepository) GetAIAccuracyStats(ctx context.Context) (models.AIFeedbackStats, error) {
	return models.AIFeedbackStats{}, nil
}

func (r *fakeRepository) GetAIRequestByID(ctx context.Context, id string) (*models.AIChatRequestRecord, error) {
	return r.request, nil
}

func (r *fakeRepository) MarkAIRequestAccuracy(ctx context.Context, requestID string, isAccurate bool, feedbackNote *string) error {
	r.marked = true
	return nil
}

func TestSubmitRequiresRequestID(t *testing.T) {
	accurate := true
	service := New(&fakeRepository{})

	err := service.Submit(context.Background(), SubmitParams{IsAccurate: &accurate})
	if !errors.Is(err, ErrRequestIDRequired) {
		t.Fatalf("Submit() error = %v, want %v", err, ErrRequestIDRequired)
	}
}

func TestSubmitRejectsDuplicateFeedback(t *testing.T) {
	accurate := true
	repository := &fakeRepository{
		request: &models.AIChatRequestRecord{IsAccurate: &accurate},
	}
	service := New(repository)

	err := service.Submit(context.Background(), SubmitParams{
		RequestID:  "request-1",
		IsAccurate: &accurate,
	})
	if !errors.Is(err, ErrFeedbackAlreadyProvided) {
		t.Fatalf("Submit() error = %v, want %v", err, ErrFeedbackAlreadyProvided)
	}
	if repository.marked {
		t.Fatal("duplicate feedback should not be marked")
	}
}

func TestSubmitMarksFeedback(t *testing.T) {
	accurate := true
	repository := &fakeRepository{request: &models.AIChatRequestRecord{}}
	service := New(repository)

	err := service.Submit(context.Background(), SubmitParams{
		RequestID:  "request-1",
		IsAccurate: &accurate,
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if !repository.marked {
		t.Fatal("expected feedback to be marked")
	}
}
