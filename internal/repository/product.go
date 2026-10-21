package repository

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type Product struct {
	ID          uuid.UUID
	Name        string
	Description *string
	Price       float64
	Stock       int
	Category    string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ProductRepo struct {
	DB *sql.DB
}

func NewProductRepo(db *sql.DB) *ProductRepo {
	return &ProductRepo{DB: db}
}

func (r *ProductRepo) Create(p *Product) error {
	err := r.DB.QueryRow(`INSERT INTO products (name, description, price, stock, category, status) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, created_at, updated_at`,
		p.Name, p.Description, p.Price, p.Stock, p.Category, p.Status).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	return err
}

func (r *ProductRepo) GetByID(id uuid.UUID) (*Product, error) {
	p := &Product{}
	var desc sql.NullString
	err := r.DB.QueryRow(`SELECT id, name, description, price, stock, category, status, created_at, updated_at FROM products WHERE id=$1`, id).
		Scan(&p.ID, &p.Name, &desc, &p.Price, &p.Stock, &p.Category, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if desc.Valid {
		p.Description = &desc.String
	}
	return p, nil
}

func (r *ProductRepo) List(page, size int, status, category *string) ([]Product, int64, error) {
	var total int64
	baseCount := `SELECT COUNT(*) FROM products WHERE 1=1`
	base := `SELECT id, name, description, price, stock, category, status, created_at, updated_at FROM products WHERE 1=1`
	args := []interface{}{}
	idx := 1
	if status != nil {
		baseCount += ` AND status = $` + itoa(idx)
		base += ` AND status = $` + itoa(idx)
		args = append(args, *status)
		idx++
	}
	if category != nil {
		baseCount += ` AND category = $` + itoa(idx)
		base += ` AND category = $` + itoa(idx)
		args = append(args, *category)
		idx++
	}
	if err := r.DB.QueryRow(baseCount, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	base += ` ORDER BY created_at DESC LIMIT $` + itoa(idx) + ` OFFSET $` + itoa(idx+1)
	args2 := append(args, size, page*size)
	rows, err := r.DB.Query(base, args2...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var list []Product
	for rows.Next() {
		var p Product
		var desc sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &desc, &p.Price, &p.Stock, &p.Category, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if desc.Valid {
			p.Description = &desc.String
		}
		list = append(list, p)
	}
	return list, total, nil
}

func (r *ProductRepo) Update(id uuid.UUID, p *Product) error {
	_, err := r.DB.Exec(`UPDATE products SET name=$1, description=$2, price=$3, stock=$4, category=$5, status=$6 WHERE id=$7`,
		p.Name, p.Description, p.Price, p.Stock, p.Category, p.Status, id)
	return err
}

func (r *ProductRepo) Archive(id uuid.UUID) error {
	_, err := r.DB.Exec(`UPDATE products SET status='ARCHIVED' WHERE id=$1`, id)
	return err
}

func itoa(i int) string {
	if i == 1 {
		return "1"
	}
	if i == 2 {
		return "2"
	}
	if i == 3 {
		return "3"
	}
	if i == 4 {
		return "4"
	}
	if i == 5 {
		return "5"
	}
	if i == 6 {
		return "6"
	}
	return "1"
}
