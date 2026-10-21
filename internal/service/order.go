package service

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"marketplace-orders/internal/repository"
)

// бизнес-ошибка с HTTP-статусом, error_code и details
type BizError struct {
	Status  int
	Code    string
	Msg     string
	Details map[string]interface{}
}

func (e *BizError) Error() string { return e.Msg }

func BizErr(status int, code, msg string) *BizError {
	return &BizError{Status: status, Code: code, Msg: msg}
}

type OrderService struct {
	orders       *repository.OrderRepo
	promos       *repository.PromoRepo
	LimitMinutes int // N минут между операциями CREATE/UPDATE
}

func NewOrderService(orders *repository.OrderRepo, promos *repository.PromoRepo) *OrderService {
	limit := 2
	if v := os.Getenv("ORDER_LIMIT_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	return &OrderService{orders: orders, promos: promos, LimitMinutes: limit}
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// создание заказа
func (s *OrderService) CreateOrder(userID uuid.UUID, items []repository.ProductQty, promoCode string) (*repository.Order, []repository.OrderItem, error) {
	// ограничение частоты создания
	if last, ok, _ := s.orders.LastOperation(nil, userID, "CREATE_ORDER"); ok {
		if time.Since(last) < time.Duration(s.LimitMinutes)*time.Minute {
			return nil, nil, BizErr(429, "ORDER_LIMIT_EXCEEDED",
				fmt.Sprintf("повторное создание заказа возможно через %d мин", s.LimitMinutes))
		}
	}
	// проверка активных заказов
	if has, _ := s.orders.HasActive(nil, userID); has {
		return nil, nil, BizErr(409, "ORDER_HAS_ACTIVE",
			"у пользователя уже есть активный заказ (CREATED/PAYMENT_PENDING)")
	}

	tx, err := s.orders.DB.Begin()
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	// проверка каталога (существует, ACTIVE) с блокировкой FOR UPDATE
	infos, err := loadProducts(tx, s.orders, items)
	if err != nil {
		return nil, nil, err
	}
	// проверка остатков
	if err := checkStock(items, infos); err != nil {
		return nil, nil, err
	}

	// снапшот цен и расчёт стоимости
	total := 0.0
	priceByID := map[uuid.UUID]float64{}
	for _, in := range items {
		for _, pi := range infos {
			if pi.ProductID == in.ProductID {
				priceByID[in.ProductID] = pi.Price
				total += pi.Price * float64(in.Quantity)
			}
		}
	}
	total = round2(total)

	// промокод
	discount := 0.0
	var promoID *uuid.UUID
	if promoCode != "" {
		pc, err := s.promos.GetByCode(tx, promoCode)
		if err != nil {
			return nil, nil, err
		}
		if pc == nil || !pc.Active || pc.CurrentUses >= pc.MaxUses ||
			time.Now().Before(pc.ValidFrom) || time.Now().After(pc.ValidUntil) {
			return nil, nil, BizErr(422, "PROMO_CODE_INVALID",
				"промокод не найден, истёк, исчерпан или неактивен")
		}
		if total < pc.MinOrderAmount {
			return nil, nil, BizErr(422, "PROMO_CODE_MIN_AMOUNT",
				fmt.Sprintf("сумма %.2f ниже минимальной %.2f", total, pc.MinOrderAmount))
		}
		if pc.DiscountType == "PERCENTAGE" {
			discount = total * pc.DiscountValue / 100
			if max := total * 0.70; discount > max {
				discount = max // не более 70% от суммы
			}
		} else {
			discount = math.Min(pc.DiscountValue, total)
		}
		discount = round2(discount)
		promoID = &pc.ID
	}
	totalAmount := round2(total - discount)

	// резерв остатков + записи заказа в одной транзакции
	order := &repository.Order{
		UserID:         userID,
		Status:         "CREATED",
		PromoCodeID:    promoID,
		TotalAmount:    totalAmount,
		DiscountAmount: discount,
	}
	if err := s.orders.Insert(tx, order); err != nil {
		return nil, nil, err
	}
	var saved []repository.OrderItem
	for _, in := range items {
		p := findProduct(infos, in.ProductID)
		ok, err := s.orders.ReserveStock(tx, in.ProductID, in.Quantity)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, &BizError{Status: 409, Code: "INSUFFICIENT_STOCK",
				Msg: "недостаточно товара на складе",
				Details: map[string]interface{}{"items": []map[string]interface{}{{
					"product_id": in.ProductID.String(), "requested": in.Quantity, "available": p.Stock,
				}}}}
		}
		it := &repository.OrderItem{OrderID: order.ID, ProductID: in.ProductID,
			Quantity: in.Quantity, PriceAtOrder: priceByID[in.ProductID]}
		if err := s.orders.InsertItem(tx, it); err != nil {
			return nil, nil, err
		}
		saved = append(saved, *it)
	}
	if promoID != nil {
		if err := s.promos.IncUses(tx, *promoID); err != nil {
			return nil, nil, err
		}
	}
	// фиксация операции CREATE_ORDER
	if err := s.orders.InsertOperation(tx, userID, "CREATE_ORDER"); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return order, saved, nil
}

// обновление заказа
func (s *OrderService) UpdateOrder(userID, orderID uuid.UUID, items []repository.ProductQty) (*repository.Order, []repository.OrderItem, error) {
	tx, err := s.orders.DB.Begin()
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	// проверка владельца и существования
	o, err := s.orders.GetForUpdate(tx, orderID)
	if err != nil {
		return nil, nil, err
	}
	if o == nil {
		return nil, nil, BizErr(404, "ORDER_NOT_FOUND", "заказ не найден")
	}
	if o.UserID != userID {
		return nil, nil, BizErr(403, "ORDER_OWNERSHIP_VIOLATION", "заказ принадлежит другому пользователю")
	}
	// проверка состояния: только CREATED
	if o.Status != "CREATED" {
		return nil, nil, BizErr(409, "INVALID_STATE_TRANSITION", "обновление разрешено только в статусе CREATED")
	}
	// ограничение частоты обновления
	if last, ok, _ := s.orders.LastOperation(nil, userID, "UPDATE_ORDER"); ok {
		if time.Since(last) < time.Duration(s.LimitMinutes)*time.Minute {
			return nil, nil, BizErr(429, "ORDER_LIMIT_EXCEEDED",
				fmt.Sprintf("повторное обновление заказа возможно через %d мин", s.LimitMinutes))
		}
	}

	// возврат предыдущих остатков
	old, err := s.orders.Items(orderID)
	if err != nil {
		return nil, nil, err
	}
	for _, it := range old {
		if err := s.orders.ReturnStock(tx, it.ProductID, it.Quantity); err != nil {
			return nil, nil, err
		}
	}

	// проверка и резервирование новых позиций
	infos, err := loadProducts(tx, s.orders, items)
	if err != nil {
		return nil, nil, err
	}
	if err := checkStock(items, infos); err != nil {
		return nil, nil, err
	}
	if err := s.orders.DeleteItems(tx, orderID); err != nil {
		return nil, nil, err
	}
	total := 0.0
	var saved []repository.OrderItem
	for _, in := range items {
		p := findProduct(infos, in.ProductID)
		ok, err := s.orders.ReserveStock(tx, in.ProductID, in.Quantity)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, &BizError{Status: 409, Code: "INSUFFICIENT_STOCK",
				Msg: "недостаточно товара на складе",
				Details: map[string]interface{}{"items": []map[string]interface{}{{
					"product_id": in.ProductID.String(), "requested": in.Quantity, "available": p.Stock,
				}}}}
		}
		it := &repository.OrderItem{OrderID: orderID, ProductID: in.ProductID,
			Quantity: in.Quantity, PriceAtOrder: p.Price}
		if err := s.orders.InsertItem(tx, it); err != nil {
			return nil, nil, err
		}
		total += p.Price * float64(in.Quantity)
		saved = append(saved, *it)
	}
	total = round2(total)

	// пересчёт стоимости, если был промокод - условия проверяются заново
	discount := 0.0
	promoID := o.PromoCodeID
	if promoID != nil {
		pc, err := s.promos.GetByID(tx, *promoID)
		if err != nil {
			return nil, nil, err
		}
		appliable := pc != nil && pc.Active && pc.CurrentUses < pc.MaxUses &&
			!time.Now().Before(pc.ValidFrom) && !time.Now().After(pc.ValidUntil)
		if appliable && total < pc.MinOrderAmount {
			appliable = false // промокод больше не применим - снимается
		}
		if appliable {
			if pc.DiscountType == "PERCENTAGE" {
				discount = total * pc.DiscountValue / 100
				if max := total * 0.70; discount > max {
					discount = max
				}
			} else {
				discount = math.Min(pc.DiscountValue, total)
			}
			discount = round2(discount)
		} else {
			promoID = nil
		}
	}
	totalAmount := round2(total - discount)
	if err := s.orders.SetTotals(tx, orderID, totalAmount, discount, promoID); err != nil {
		return nil, nil, err
	}
	if promoID == nil && o.PromoCodeID != nil {
		if err := s.promos.DecUses(tx, *o.PromoCodeID); err != nil {
			return nil, nil, err
		}
	}

	// фиксация операции UPDATE_ORDER
	if err := s.orders.InsertOperation(tx, userID, "UPDATE_ORDER"); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	nu, _ := s.orders.GetByID(orderID)
	ni, _ := s.orders.Items(orderID)
	return nu, ni, nil
}

// отмена заказа
func (s *OrderService) CancelOrder(userID, orderID uuid.UUID) (*repository.Order, error) {
	tx, err := s.orders.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// проверка владельца
	o, err := s.orders.GetForUpdate(tx, orderID)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, BizErr(404, "ORDER_NOT_FOUND", "заказ не найден")
	}
	if o.UserID != userID {
		return nil, BizErr(403, "ORDER_OWNERSHIP_VIOLATION", "заказ принадлежит другому пользователю")
	}
	// проверка состояния
	if o.Status != "CREATED" && o.Status != "PAYMENT_PENDING" {
		return nil, BizErr(409, "INVALID_STATE_TRANSITION",
			"отмена разрешена только из CREATED/PAYMENT_PENDING")
	}
	// возврат остатков
	items, err := s.orders.Items(orderID)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		if err := s.orders.ReturnStock(tx, it.ProductID, it.Quantity); err != nil {
			return nil, err
		}
	}
	// возврат использования промокода
	if o.PromoCodeID != nil {
		if err := s.promos.DecUses(tx, *o.PromoCodeID); err != nil {
			return nil, err
		}
	}
	// установка статуса
	if err := s.orders.SetStatus(tx, orderID, "CANCELED"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.orders.GetByID(orderID)
}

// вспомогательные методы для хендлеров
func (s *OrderService) GetOrder(id uuid.UUID) (*repository.Order, error) {
	return s.orders.GetByID(id)
}

func (s *OrderService) OrderItems(id uuid.UUID) ([]repository.OrderItem, error) {
	return s.orders.Items(id)
}

func (s *OrderService) GetPromo(id uuid.UUID) (*repository.PromoCode, error) {
	return s.promos.GetByID(nil, id)
}

func (s *OrderService) GetPromoByCode(code string) (*repository.PromoCode, error) {
	return s.promos.GetByCode(nil, code)
}

func (s *OrderService) CreatePromo(pc *repository.PromoCode) error {
	return s.promos.Create(pc)
}

// общие проверки каталога и остатков
func loadProducts(tx *sql.Tx, r *repository.OrderRepo, items []repository.ProductQty) ([]repository.ProductInfo, error) {
	ids := map[uuid.UUID]bool{}
	var list []uuid.UUID
	for _, in := range items {
		if !ids[in.ProductID] {
			ids[in.ProductID] = true
			list = append(list, in.ProductID)
		}
	}
	infos, err := r.ProductsForUpdate(tx, list)
	if err != nil {
		return nil, err
	}
	byID := map[uuid.UUID]repository.ProductInfo{}
	for _, pi := range infos {
		byID[pi.ProductID] = pi
	}
	for _, in := range items {
		p, ok := byID[in.ProductID]
		if !ok {
			return nil, BizErr(404, "PRODUCT_NOT_FOUND", fmt.Sprintf("товар %s не найден", in.ProductID))
		}
		if p.Status != "ACTIVE" {
			return nil, BizErr(409, "PRODUCT_INACTIVE",
				fmt.Sprintf("товар %s неактивен (статус %s)", in.ProductID, p.Status))
		}
	}
	return infos, nil
}

func checkStock(items []repository.ProductQty, infos []repository.ProductInfo) error {
	byID := map[uuid.UUID]repository.ProductInfo{}
	for _, pi := range infos {
		byID[pi.ProductID] = pi
	}
	var missing []map[string]interface{}
	for _, in := range items {
		p := byID[in.ProductID]
		if p.Stock < in.Quantity {
			missing = append(missing, map[string]interface{}{
				"product_id": in.ProductID.String(),
				"requested":  in.Quantity,
				"available":  p.Stock,
			})
		}
	}
	if len(missing) > 0 {
		return &BizError{Status: 409, Code: "INSUFFICIENT_STOCK",
			Msg:     "недостаточно товара на складе",
			Details: map[string]interface{}{"items": missing}}
	}
	return nil
}

func findProduct(infos []repository.ProductInfo, id uuid.UUID) repository.ProductInfo {
	for _, pi := range infos {
		if pi.ProductID == id {
			return pi
		}
	}
	return repository.ProductInfo{}
}
