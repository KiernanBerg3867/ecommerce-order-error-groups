package orderflow

import (
	"context"
	"fmt"
	"strings"

	"example.com/ecommerce-order-errors/infrai"
)

type ErrorTracker interface {
	Capture(context.Context, infrai.CaptureInput, string) (infrai.CaptureResult, error)
	GroupDetail(context.Context, string) (infrai.GroupDetail, error)
}

type OrderFailure struct {
	OrderID   string `json:"order_id"`
	Stage     string `json:"stage"`
	Operation string `json:"operation"`
	Message   string `json:"message"`
	ReceiptID string `json:"receipt_id,omitempty"`
	Customer  string `json:"customer,omitempty"`
}

type UpdateResult struct {
	OrderID      string `json:"order_id"`
	State        string `json:"state"`
	ErrorGroupID string `json:"error_group_id"`
	Occurrences  int    `json:"occurrences"`
}

func RecordFailure(ctx context.Context, tracker ErrorTracker, failure OrderFailure) (UpdateResult, error) {
	stage, err := normalizeStage(failure.Stage)
	if err != nil {
		return UpdateResult{}, err
	}
	operation := strings.TrimSpace(failure.Operation)
	if operation == "" {
		return UpdateResult{}, fmt.Errorf("operation is required")
	}

	fingerprint := []string{"ecommerce-order", stage, operation}
	captured, err := tracker.Capture(ctx, infrai.CaptureInput{
		Title:       fmt.Sprintf("%s %s failed", stage, operation),
		Message:     failure.Message,
		Exception:   failure.Message,
		Level:       "error",
		Tags:        map[string]any{"stage": stage, "operation": operation},
		Fingerprint: fingerprint,
		Context: map[string]any{
			"order_id":   failure.OrderID,
			"receipt_id": failure.ReceiptID,
			"customer":   failure.Customer,
		},
	}, "order-error:"+failure.OrderID+":"+stage+":"+operation)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("capture order failure: %w", err)
	}

	group, err := tracker.GroupDetail(ctx, captured.ErrorGroupID)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("read error group: %w", err)
	}
	return UpdateResult{
		OrderID:      failure.OrderID,
		State:        "attention_required",
		ErrorGroupID: captured.ErrorGroupID,
		Occurrences:  group.Count,
	}, nil
}

func normalizeStage(value string) (string, error) {
	stage := strings.ToLower(strings.TrimSpace(value))
	switch stage {
	case "checkout", "fulfillment", "receipt", "customer_update":
		return stage, nil
	default:
		return "", fmt.Errorf("stage must be checkout, fulfillment, receipt, or customer_update")
	}
}
