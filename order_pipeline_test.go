package main

import "testing"

func TestFulfillmentProducesReceiptAndCustomerUpdate(t *testing.T) {
	tests := []struct {
		name      string
		checkout  bool
		wantError bool
	}{
		{name: "unknown order has no receipt", checkout: false, wantError: true},
		{name: "checked out order produces matching records", checkout: true, wantError: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := NewOrderPipeline()
			if tt.checkout {
				_, err := pipeline.Checkout(Order{ID: "ord-1042", Email: "buyer@example.com", SKU: "mug-blue", Quantity: 2})
				if err != nil {
					t.Fatal(err)
				}
			}
			receipt, update, err := pipeline.Fulfill("ord-1042")
			if (err != nil) != tt.wantError {
				t.Fatalf("Fulfill() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError {
				if _, exists := pipeline.Receipt("ord-1042"); exists {
					t.Fatal("receipt exists before a valid checkout")
				}
				return
			}
			if receipt.OrderID != update.OrderID || update.Status != StatusFulfilled {
				t.Fatalf("handoff mismatch: receipt=%+v update=%+v", receipt, update)
			}
		})
	}
}
