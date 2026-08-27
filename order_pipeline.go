package main

import (
	"errors"
	"fmt"
	"sync"
)

type OrderStatus string

const (
	StatusCheckedOut OrderStatus = "checked_out"
	StatusFulfilled  OrderStatus = "fulfilled"
)

type Order struct {
	ID       string      `json:"id"`
	Email    string      `json:"email"`
	SKU      string      `json:"sku"`
	Quantity int         `json:"quantity"`
	Status   OrderStatus `json:"status"`
}

type Receipt struct {
	OrderID string `json:"order_id"`
	Number  string `json:"number"`
}

type CustomerUpdate struct {
	OrderID string      `json:"order_id"`
	Email   string      `json:"email"`
	Status  OrderStatus `json:"status"`
}

type OrderPipeline struct {
	mu       sync.Mutex
	orders   map[string]Order
	receipts map[string]Receipt
	updates  []CustomerUpdate
}

func NewOrderPipeline() *OrderPipeline {
	return &OrderPipeline{orders: make(map[string]Order), receipts: make(map[string]Receipt)}
}

func (p *OrderPipeline) Checkout(order Order) (Order, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if order.ID == "" || order.SKU == "" || order.Quantity < 1 {
		return Order{}, errors.New("order id, sku, and positive quantity are required")
	}
	if _, exists := p.orders[order.ID]; exists {
		return p.orders[order.ID], nil
	}
	order.Status = StatusCheckedOut
	p.orders[order.ID] = order
	return order, nil
}

func (p *OrderPipeline) Fulfill(orderID string) (Receipt, CustomerUpdate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	order, exists := p.orders[orderID]
	if !exists {
		return Receipt{}, CustomerUpdate{}, errors.New("order not found")
	}
	if receipt, done := p.receipts[orderID]; done {
		return receipt, CustomerUpdate{OrderID: order.ID, Email: order.Email, Status: StatusFulfilled}, nil
	}
	order.Status = StatusFulfilled
	p.orders[orderID] = order
	receipt := Receipt{OrderID: order.ID, Number: fmt.Sprintf("R-%s", order.ID)}
	update := CustomerUpdate{OrderID: order.ID, Email: order.Email, Status: order.Status}
	p.receipts[orderID] = receipt
	p.updates = append(p.updates, update)
	return receipt, update, nil
}

func (p *OrderPipeline) Receipt(orderID string) (Receipt, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	receipt, ok := p.receipts[orderID]
	return receipt, ok
}
