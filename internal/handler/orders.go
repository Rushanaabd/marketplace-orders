package handler

import (
	"errors"
	"net/http"

	gen "marketplace-orders/internal/api/gen"
	"marketplace-orders/internal/repository"
	"marketplace-orders/internal/service"
	"marketplace-orders/internal/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// превращает бизнес-ошибку сервиса в единый формат ErrorResponse
func fail(c *gin.Context, err error) {
	var be *service.BizError
	if errors.As(err, &be) {
		util.Error(c, be.Status, be.Code, be.Msg, be.Details)
		return
	}
	util.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error", nil)
}

func validationErr(c *gin.Context, v map[string]string) {
	details := map[string]interface{}{}
	for f, m := range v {
		details[f] = m
	}
	util.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "ошибка валидации входных данных", details)
}

func toQty(items []gen.OrderItemInput) []repository.ProductQty {
	qty := make([]repository.ProductQty, 0, len(items))
	for _, in := range items {
		qty = append(qty, repository.ProductQty{ProductID: uuid.UUID(in.ProductId), Quantity: in.Quantity})
	}
	return qty
}

func orderResponse(o *repository.Order, items []repository.OrderItem, promoCode *string) gen.OrderResponse {
	resp := gen.OrderResponse{
		Id:             openapi_types.UUID(o.ID),
		UserId:         openapi_types.UUID(o.UserID),
		Status:         gen.OrderStatus(o.Status),
		Items:          []gen.OrderItemResponse{},
		TotalAmount:    o.TotalAmount,
		DiscountAmount: o.DiscountAmount,
		CreatedAt:      o.CreatedAt,
		UpdatedAt:      o.UpdatedAt,
	}
	resp.PromoCode = promoCode
	for _, it := range items {
		resp.Items = append(resp.Items, gen.OrderItemResponse{
			Id:           openapi_types.UUID(it.ID),
			ProductId:    openapi_types.UUID(it.ProductID),
			Quantity:     it.Quantity,
			PriceAtOrder: it.PriceAtOrder,
		})
	}
	return resp
}

// CREATE /orders
func (h *Handler) CreateOrder(c *gin.Context, params gen.CreateOrderParams) {
	var req gen.OrderCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		validationErr(c, map[string]string{"body": "некорректный JSON"})
		return
	}
	if v := util.ValidateOrderItems(req); len(v) > 0 {
		validationErr(c, v)
		return
	}
	var promo *string
	if req.PromoCode != nil {
		promo = req.PromoCode
	}
	order, items, err := h.svc.CreateOrder(uuid.UUID(params.XUserId), toQty(req.Items), promoStr(promo))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, orderResponse(order, items, promo))
}

func promoStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// GET /orders/{id}
func (h *Handler) GetOrder(c *gin.Context, id openapi_types.UUID, params gen.GetOrderParams) {
	o, err := h.svc.GetOrder(uuid.UUID(id))
	if err != nil {
		fail(c, err)
		return
	}
	if o == nil {
		fail(c, service.BizErr(404, "ORDER_NOT_FOUND", "заказ не найден"))
		return
	}
	if o.UserID != uuid.UUID(params.XUserId) {
		fail(c, service.BizErr(403, "ORDER_OWNERSHIP_VIOLATION", "заказ принадлежит другому пользователю"))
		return
	}
	items, err := h.svc.OrderItems(uuid.UUID(id))
	if err != nil {
		fail(c, err)
		return
	}
	var promo *string
	if o.PromoCodeID != nil {
		if pc, _ := h.svc.GetPromo(*o.PromoCodeID); pc != nil {
			promo = &pc.Code
		}
	}
	c.JSON(http.StatusOK, orderResponse(o, items, promo))
}

// PUT /orders/{id}
func (h *Handler) UpdateOrder(c *gin.Context, id openapi_types.UUID, params gen.UpdateOrderParams) {
	var req gen.OrderUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		validationErr(c, map[string]string{"body": "некорректный JSON"})
		return
	}
	if v := util.ValidateOrderItems(gen.OrderCreate{Items: req.Items}); len(v) > 0 {
		validationErr(c, v)
		return
	}
	order, items, err := h.svc.UpdateOrder(uuid.UUID(params.XUserId), uuid.UUID(id), toQty(req.Items))
	if err != nil {
		fail(c, err)
		return
	}
	var promo *string
	if order.PromoCodeID != nil {
		if pc, _ := h.svc.GetPromo(*order.PromoCodeID); pc != nil {
			promo = &pc.Code
		}
	}
	c.JSON(http.StatusOK, orderResponse(order, items, promo))
}

// POST /orders/{id}/cancel
func (h *Handler) CancelOrder(c *gin.Context, id openapi_types.UUID, params gen.CancelOrderParams) {
	order, err := h.svc.CancelOrder(uuid.UUID(params.XUserId), uuid.UUID(id))
	if err != nil {
		fail(c, err)
		return
	}
	items, _ := h.svc.OrderItems(uuid.UUID(id))
	var promo *string
	if order.PromoCodeID != nil {
		if pc, _ := h.svc.GetPromo(*order.PromoCodeID); pc != nil {
			promo = &pc.Code
		}
	}
	c.JSON(http.StatusOK, orderResponse(order, items, promo))
}

// POST /promo-codes
func (h *Handler) CreatePromoCode(c *gin.Context) {
	var req gen.PromoCodeCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		validationErr(c, map[string]string{"body": "некорректный JSON"})
		return
	}
	newUuid, err := uuid.NewRandom()
	if err != nil {
		fail(c, err)
		return
	}
	pc := &repository.PromoCode{
		ID:             newUuid,
		Code:           req.Code,
		DiscountType:   string(req.DiscountType),
		DiscountValue:  req.DiscountValue,
		MinOrderAmount: req.MinOrderAmount,
		MaxUses:        req.MaxUses,
		ValidFrom:      req.ValidFrom,
		ValidUntil:     req.ValidUntil,
		Active:         true,
	}
	if req.Active != nil {
		pc.Active = *req.Active
	}
	if ex, _ := h.svc.GetPromoByCode(req.Code); ex != nil {
		validationErr(c, map[string]string{"code": "промокод с таким кодом уже существует"})
		return
	}
	if err := h.svc.CreatePromo(pc); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, promoResponse(pc))
}

func promoResponse(pc *repository.PromoCode) gen.PromoCodeResponse {
	return gen.PromoCodeResponse{
		Id:             openapi_types.UUID(pc.ID),
		Code:           pc.Code,
		DiscountType:   gen.PromoCodeResponseDiscountType(pc.DiscountType),
		DiscountValue:  pc.DiscountValue,
		MinOrderAmount: pc.MinOrderAmount,
		MaxUses:        pc.MaxUses,
		CurrentUses:    pc.CurrentUses,
		ValidFrom:      pc.ValidFrom,
		ValidUntil:     pc.ValidUntil,
		Active:         pc.Active,
	}
}
