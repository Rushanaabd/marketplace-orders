package repository

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Querier interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

func q(db *sql.DB, tx *sql.Tx) Querier {
	if tx != nil {
		return tx
	}
	return db
}

type Order struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Status         string
	PromoCodeID    *uuid.UUID
	TotalAmount    float64
	DiscountAmount float64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type OrderItem struct {
	ID           uuid.UUID
	OrderID      uuid.UUID
	ProductID    uuid.UUID
	Quantity     int
	PriceAtOrder float64
}

type ProductQty struct {
	ProductID uuid.UUID
	Quantity  int
}

type ProductInfo struct {
	ProductID uuid.UUID
	Price     float64
	Stock     int
	Status    string
}

type OrderRepo struct{ DB *sql.DB }

func NewOrderRepo(db *sql.DB) *OrderRepo { return &OrderRepo{DB: db} }

func (r *OrderRepo) LastOperation(tx *sql.Tx, userID uuid.UUID, opType string) (time.Time, bool, error) {
	var t time.Time
	err := q(r.DB, tx).QueryRow(`SELECT created_at FROM user_operations
		WHERE user_id=$1 AND operation_type=$2 ORDER BY created_at DESC LIMIT 1`,
		userID, opType).Scan(&t)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return t, true, nil
}

func (r *OrderRepo) InsertOperation(tx *sql.Tx, userID uuid.UUID, opType string) error {
	_, err := tx.Exec(`INSERT INTO user_operations (user_id, operation_type) VALUES ($1,$2)`,
		userID, opType)
	return err
}

func (r *OrderRepo) HasActive(tx *sql.Tx, userID uuid.UUID) (bool, error) {
	var n int
	err := q(r.DB, tx).QueryRow(`SELECT COUNT(*) FROM orders
		WHERE user_id=$1 AND status IN ('CREATED','PAYMENT_PENDING')`, userID).Scan(&n)
	return n > 0, err
}

func (r *OrderRepo) GetByID(id uuid.UUID) (*Order, error) {
	return getOrder(r.DB, id, false)
}

func (r *OrderRepo) GetForUpdate(tx *sql.Tx, id uuid.UUID) (*Order, error) {
	return getOrder(tx, id, true)
}

func getOrder(db Querier, id uuid.UUID, lock bool) (*Order, error) {
	o := &Order{}
	var promo sql.NullString
	sel := `SELECT id, user_id, status, promo_code_id, total_amount, discount_amount, created_at, updated_at
		FROM orders WHERE id=$1`
	if lock {
		sel += ` FOR UPDATE`
	}
	err := db.QueryRow(sel, id).
		Scan(&o.ID, &o.UserID, &o.Status, &promo, &o.TotalAmount, &o.DiscountAmount, &o.CreatedAt, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if promo.Valid {
		u, _ := uuid.Parse(promo.String)
		o.PromoCodeID = &u
	}
	return o, nil
}

func (r *OrderRepo) Items(orderID uuid.UUID) ([]OrderItem, error) {
	rows, err := r.DB.Query(`SELECT id, order_id, product_id, quantity, price_at_order
		FROM order_items WHERE order_id=$1`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []OrderItem
	for rows.Next() {
		var it OrderItem
		if err := rows.Scan(&it.ID, &it.OrderID, &it.ProductID, &it.Quantity, &it.PriceAtOrder); err != nil {
			return nil, err
		}
		list = append(list, it)
	}
	return list, rows.Err()
}

func (r *OrderRepo) Insert(tx *sql.Tx, o *Order) error {
	return tx.QueryRow(`INSERT INTO orders (user_id, status, promo_code_id, total_amount, discount_amount)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at, updated_at`,
		o.UserID, o.Status, nullableUUID(o.PromoCodeID), o.TotalAmount, o.DiscountAmount).
		Scan(&o.ID, &o.CreatedAt, &o.UpdatedAt)
}

func (r *OrderRepo) InsertItem(tx *sql.Tx, it *OrderItem) error {
	return tx.QueryRow(`INSERT INTO order_items (order_id, product_id, quantity, price_at_order)
		VALUES ($1,$2,$3,$4) RETURNING id`,
		it.OrderID, it.ProductID, it.Quantity, it.PriceAtOrder).Scan(&it.ID)
}

func (r *OrderRepo) DeleteItems(tx *sql.Tx, orderID uuid.UUID) error {
	_, err := tx.Exec(`DELETE FROM order_items WHERE order_id=$1`, orderID)
	return err
}

func (r *OrderRepo) SetStatus(tx *sql.Tx, id uuid.UUID, status string) error {
	_, err := tx.Exec(`UPDATE orders SET status=$2 WHERE id=$1`, id, status)
	return err
}

func (r *OrderRepo) SetTotals(tx *sql.Tx, id uuid.UUID, total, discount float64, promoID *uuid.UUID) error {
	_, err := tx.Exec(`UPDATE orders SET total_amount=$2, discount_amount=$3, promo_code_id=$4 WHERE id=$1`,
		id, total, discount, nullableUUID(promoID))
	return err
}

// товары внутри транзакции (блокировка, проверка, резерв, возврат)
func (r *OrderRepo) ProductsForUpdate(tx *sql.Tx, ids []uuid.UUID) ([]ProductInfo, error) {
	rows, err := tx.Query(`SELECT id, price, stock, status FROM products WHERE id = ANY($1) FOR UPDATE`,
		pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []ProductInfo
	for rows.Next() {
		var p ProductInfo
		if err := rows.Scan(&p.ProductID, &p.Price, &p.Stock, &p.Status); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *OrderRepo) ReserveStock(tx *sql.Tx, productID uuid.UUID, qty int) (bool, error) {
	res, err := tx.Exec(`UPDATE products SET stock = stock - $2 WHERE id=$1 AND stock >= $2`,
		productID, qty)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *OrderRepo) ReturnStock(tx *sql.Tx, productID uuid.UUID, qty int) error {
	_, err := tx.Exec(`UPDATE products SET stock = stock + $2 WHERE id=$1`, productID, qty)
	return err
}

func nullableUUID(u *uuid.UUID) interface{} {
	if u == nil {
		return nil
	}
	return *u
}
