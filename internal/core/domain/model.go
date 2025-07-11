package domain

type Order struct {
	ID              int        `json:"id"`
	Number          int        `json:"number"`
	CustomerName    string     `json:"customer_name"`
	Type            string     `json:"type"`
	TableNumber     *int       `json:"table_number,omitempty"`
	DeliveryAddress *string    `json:"delivery_address,omitempty"`
	TotalAmount     float64    `json:"total_amount"`
	Priority        int        `json:"priority"`
	Status          string     `json:"status"`
	ProcessedBy     *string    `json:"processed_by,omitempry"`
	CompletedAt     *time.Time `json:"completed_at,omitempry"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type orderItem struct {
	ID        int       `json:"id"`
	OrderID   int       `json:"order_id"`
	Name      string    `json:"name"`
	Quantity  int       `json:"quantity"`
	Price     float64   `json:"price"`
	CreatedAt time.Time `json:"created_at"`
}

type OrderStatusLog struct {
	ID        int       `json:"id"`
	OrderID   int       `json:"order_id"`
	Status    string    `json:"status"`
	ChangedBy *string   `json:"changed_by"`
	Notes     *string   `json:"notes"`
	ChangedAt time.Time `json:"changed_at"`
	CreatedAt time.Time `json:"created_at"`
}

type Worker struct {
	ID              int       `json:"id"`
	Name            string    `json:"name"`
	Type            string    `json:"type"`
	Status          string    `json:"status"`
	LastSeen        time.Time `json:"last_seen"`
	OrdersProcessed int       `json:"orders_processed"`
	CreatedAt       time.Time `json:"created_at"`
}
