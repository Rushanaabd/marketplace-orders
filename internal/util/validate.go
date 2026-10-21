package util

import (
	"regexp"
	"strconv"
	"unicode/utf8"

	gen "marketplace-orders/internal/api/gen"
)

const promoPattern = `^[A-Z0-9_]{4,20}$`

// проверяет констрейнты из OpenAPI до бизнес-логики
func ValidateProductCreate(in gen.ProductCreate) map[string]string {
	v := map[string]string{}
	if l := utf8.RuneCountInString(in.Name); l < 1 || l > 255 {
		v["name"] = "длина от 1 до 255"
	}
	if in.Description != nil && utf8.RuneCountInString(*in.Description) > 4000 {
		v["description"] = "максимальная длина 4000"
	}
	if in.Price < 0.01 {
		v["price"] = "минимум 0.01"
	}
	if in.Stock < 0 {
		v["stock"] = "минимум 0"
	}
	if l := utf8.RuneCountInString(in.Category); l < 1 || l > 100 {
		v["category"] = "длина от 1 до 100"
	}
	if !in.Status.Valid() {
		v["status"] = "одно из ACTIVE, INACTIVE, ARCHIVED"
	}
	return v
}

// проверяет опциональные поля обновления
func ValidateProductUpdate(in gen.ProductUpdate) map[string]string {
	v := map[string]string{}
	if in.Name != nil {
		if l := utf8.RuneCountInString(*in.Name); l < 1 || l > 255 {
			v["name"] = "длина от 1 до 255"
		}
	}
	if in.Description != nil && utf8.RuneCountInString(*in.Description) > 4000 {
		v["description"] = "максимальная длина 4000"
	}
	if in.Price != nil && *in.Price < 0.01 {
		v["price"] = "минимум 0.01"
	}
	if in.Stock != nil && *in.Stock < 0 {
		v["stock"] = "минимум 0"
	}
	if in.Category != nil {
		if l := utf8.RuneCountInString(*in.Category); l < 1 || l > 100 {
			v["category"] = "длина от 1 до 100"
		}
	}
	if in.Status != nil && !in.Status.Valid() {
		v["status"] = "одно из ACTIVE, INACTIVE, ARCHIVED"
	}
	return v
}

// проверяет items и promo_code для заказов
func ValidateOrderItems(in gen.OrderCreate) map[string]string {
	v := map[string]string{}
	if n := len(in.Items); n < 1 || n > 50 {
		v["items"] = "от 1 до 50 позиций"
	}
	for i, item := range in.Items {
		if item.Quantity < 1 || item.Quantity > 999 {
			v["items."+strconv.Itoa(i)] = "quantity от 1 до 999"
		}
	}
	if in.PromoCode != nil && !regexp.MustCompile(promoPattern).MatchString(*in.PromoCode) {
		v["promo_code"] = "регулярное выражение ^[A-Z0-9_]{4,20}$"
	}
	return v
}
