package examples_test

import (
	"context"
	"errors"
	"testing"
)

// Sentinel domain errors.
var (
	ErrInvalidAmount = errors.New("order amount must be positive")
	ErrOrderExists   = errors.New("order already exists")
)

// Domain entity.
type Order struct {
	ID     string
	Amount int64
}

// Consumer-defined interface (defined where consumed, not in provider).
type OrderRepository interface {
	Save(ctx context.Context, order Order) error
}

// Domain service.
type OrderService struct {
	repo OrderRepository
}

// NewOrderService constructor returning concrete struct.
func NewOrderService(repo OrderRepository) *OrderService {
	return &OrderService{repo: repo}
}

// CreateOrder executes business logic and persists the order.
func (s *OrderService) CreateOrder(ctx context.Context, id string, amount int64) (*Order, error) {
	if id == "" {
		return nil, errors.New("order ID cannot be empty")
	}
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}

	order := Order{ID: id, Amount: amount}
	if err := s.repo.Save(ctx, order); err != nil {
		return nil, err
	}

	return &order, nil
}

// Lightweight function-field test double (no external mock generator required).
type mockOrderRepository struct {
	saveFunc func(ctx context.Context, order Order) error
}

func (m *mockOrderRepository) Save(ctx context.Context, order Order) error {
	if m.saveFunc == nil {
		panic("mockOrderRepository.Save invoked without implementation")
	}
	return m.saveFunc(ctx, order)
}

func TestOrderService_CreateOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		orderID     string
		amount      int64
		mockRepo    OrderRepository
		wantOrder   *Order
		wantErr     bool
		expectedErr error
	}{
		{
			name:    "success: valid order created and saved",
			orderID: "ord_1001",
			amount:  5000,
			mockRepo: &mockOrderRepository{
				saveFunc: func(ctx context.Context, order Order) error {
					if order.ID != "ord_1001" || order.Amount != 5000 {
						t.Errorf("unexpected order passed to Save: %+v", order)
					}
					return nil
				},
			},
			wantOrder: &Order{ID: "ord_1001", Amount: 5000},
			wantErr:   false,
		},
		{
			name:     "error: empty order ID",
			orderID:  "",
			amount:   5000,
			mockRepo: &mockOrderRepository{},
			wantErr:  true,
		},
		{
			name:        "error: zero or negative amount",
			orderID:     "ord_1002",
			amount:      0,
			mockRepo:    &mockOrderRepository{},
			wantErr:     true,
			expectedErr: ErrInvalidAmount,
		},
		{
			name:    "error: repository returns duplicate failure",
			orderID: "ord_1003",
			amount:  2500,
			mockRepo: &mockOrderRepository{
				saveFunc: func(ctx context.Context, order Order) error {
					return ErrOrderExists
				},
			},
			wantErr:     true,
			expectedErr: ErrOrderExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := NewOrderService(tt.mockRepo)
			got, err := svc.CreateOrder(context.Background(), tt.orderID, tt.amount)

			if (err != nil) != tt.wantErr {
				t.Fatalf("CreateOrder() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
				t.Fatalf("CreateOrder() error = %v, expectedErr = %v", err, tt.expectedErr)
			}
			if !tt.wantErr {
				if got.ID != tt.wantOrder.ID || got.Amount != tt.wantOrder.Amount {
					t.Errorf("CreateOrder() got = %+v, want %+v", got, tt.wantOrder)
				}
			}
		})
	}
}
