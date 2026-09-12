package orderflow

import (
	"context"
	"reflect"
	"testing"

	"example.com/ecommerce-order-errors/infrai"
)

type trackerStub struct {
	input          infrai.CaptureInput
	idempotencyKey string
}

func (s *trackerStub) Capture(_ context.Context, input infrai.CaptureInput, key string) (infrai.CaptureResult, error) {
	s.input = input
	s.idempotencyKey = key
	return infrai.CaptureResult{EventID: "evt-1", ErrorGroupID: "grp-7"}, nil
}

func (s *trackerStub) GroupDetail(_ context.Context, groupID string) (infrai.GroupDetail, error) {
	return infrai.GroupDetail{ErrorGroupID: groupID, Status: "unresolved", Count: 4}, nil
}

func TestRecordFailureGroupsByStageAndOperation(t *testing.T) {
	tests := []struct {
		name      string
		failure   OrderFailure
		wantPrint []string
		wantKey   string
	}{
		{"checkout payment", OrderFailure{OrderID: "ord-42", Stage: "checkout", Operation: "authorize_payment", Message: "issuer declined"}, []string{"ecommerce-order", "checkout", "authorize_payment"}, "order-error:ord-42:checkout:authorize_payment"},
		{"receipt delivery", OrderFailure{OrderID: "ord-42", Stage: "receipt", Operation: "send_receipt", Message: "mailbox rejected", ReceiptID: "rcpt-9"}, []string{"ecommerce-order", "receipt", "send_receipt"}, "order-error:ord-42:receipt:send_receipt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := &trackerStub{}
			got, err := RecordFailure(context.Background(), tracker, tt.failure)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(tracker.input.Fingerprint, tt.wantPrint) {
				t.Fatalf("fingerprint = %v, want %v", tracker.input.Fingerprint, tt.wantPrint)
			}
			if tracker.idempotencyKey != tt.wantKey {
				t.Fatalf("idempotency key = %q, want %q", tracker.idempotencyKey, tt.wantKey)
			}
			if got.State != "attention_required" || got.ErrorGroupID != "grp-7" || got.Occurrences != 4 {
				t.Fatalf("result = %#v", got)
			}
		})
	}
}
