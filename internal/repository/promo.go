package repository

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type PromoCode struct {
	ID             uuid.UUID
	Code           string
	DiscountType   string
	DiscountValue  float64
	MinOrderAmount float64
	MaxUses        int
	CurrentUses    int
	ValidFrom      time.Time
	ValidUntil     time.Time
	Active         bool
}

type PromoRepo struct{ DB *sql.DB }

func NewPromoRepo(db *sql.DB) *PromoRepo { return &PromoRepo{DB: db} }

func scanPromo(row *sql.Row) (*PromoCode, error) {
	p := &PromoCode{}
	err := row.Scan(&p.ID, &p.Code, &p.DiscountType, &p.DiscountValue, &p.MinOrderAmount,
		&p.MaxUses, &p.CurrentUses, &p.ValidFrom, &p.ValidUntil, &p.Active)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *PromoRepo) GetByCode(tx *sql.Tx, code string) (*PromoCode, error) {
	return scanPromo(q(r.DB, tx).QueryRow(`SELECT id, code, discount_type, discount_value, min_order_amount,
		max_uses, current_uses, valid_from, valid_until, active FROM promo_codes WHERE code=$1`, code))
}

func (r *PromoRepo) GetByID(tx *sql.Tx, id uuid.UUID) (*PromoCode, error) {
	return scanPromo(q(r.DB, tx).QueryRow(`SELECT id, code, discount_type, discount_value, min_order_amount,
		max_uses, current_uses, valid_from, valid_until, active FROM promo_codes WHERE id=$1`, id))
}

func (r *PromoRepo) Create(p *PromoCode) error {
	return r.DB.QueryRow(`INSERT INTO promo_codes
		(code, discount_type, discount_value, min_order_amount, max_uses, valid_from, valid_until, active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		p.Code, p.DiscountType, p.DiscountValue, p.MinOrderAmount,
		p.MaxUses, p.ValidFrom, p.ValidUntil, p.Active).Scan(&p.ID)
}

func (r *PromoRepo) IncUses(tx *sql.Tx, id uuid.UUID) error {
	_, err := tx.Exec(`UPDATE promo_codes SET current_uses = current_uses + 1 WHERE id=$1`, id)
	return err
}

func (r *PromoRepo) DecUses(tx *sql.Tx, id uuid.UUID) error {
	_, err := tx.Exec(`UPDATE promo_codes SET current_uses = GREATEST(current_uses - 1, 0) WHERE id=$1`, id)
	return err
}
