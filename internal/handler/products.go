package handler

import (
	"database/sql"
	"net/http"

	gen "marketplace-orders/internal/api/gen"
	"marketplace-orders/internal/repository"
	"marketplace-orders/internal/service"
	"marketplace-orders/internal/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type Handler struct {
	repo *repository.ProductRepo
	db   *sql.DB
	svc  *service.OrderService
}

func New(repo *repository.ProductRepo, db *sql.DB) *Handler {
	return &Handler{repo: repo, db: db}
}

func (h *Handler) SetService(svc *service.OrderService) {
	h.svc = svc
}

// Проверка что Handler реализует интерфейс
var _ gen.ServerInterface = (*Handler)(nil)

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) ListProducts(c *gin.Context, params gen.ListProductsParams) {
	page := 0
	if params.Page != nil {
		page = *params.Page
	}
	size := 20
	if params.Size != nil {
		size = *params.Size
	}
	var status *string
	if params.Status != nil {
		s := string(*params.Status)
		status = &s
	}
	var cat *string
	if params.Category != nil {
		cat = params.Category
	}
	list, total, err := h.repo.List(page, size, status, cat)
	if err != nil {
		util.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error", nil)
		return
	}
	res := gen.ProductPageResponse{
		Content:       []gen.ProductResponse{},
		Page:          page,
		Size:          size,
		TotalElements: total,
	}
	for _, p := range list {
		pr := gen.ProductResponse{
			Id:       openapi_types.UUID(p.ID),
			Name:     p.Name,
			Price:    p.Price,
			Stock:    p.Stock,
			Category: p.Category,
			Status:   gen.ProductStatus(p.Status),
		}
		if p.Description != nil {
			d := *p.Description
			pr.Description = &d
		}
		ct := p.CreatedAt
		ut := p.UpdatedAt
		pr.CreatedAt = ct
		pr.UpdatedAt = ut
		res.Content = append(res.Content, pr)
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) CreateProduct(c *gin.Context) {
	var req gen.ProductCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "ошибка валидации входных данных", nil)
		return
	}
	if v := util.ValidateProductCreate(req); len(v) > 0 {
		util.ValidationDetails(c, v)
		return
	}
	var desc *string
	if req.Description != nil {
		desc = req.Description
	}
	p := &repository.Product{
		Name:        req.Name,
		Description: desc,
		Price:       req.Price,
		Stock:       req.Stock,
		Category:    req.Category,
		Status:      string(req.Status),
	}
	if err := h.repo.Create(p); err != nil {
		util.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "validation error", nil)
		return
	}
	pr := gen.ProductResponse{
		Id:        openapi_types.UUID(p.ID),
		Name:      p.Name,
		Price:     p.Price,
		Stock:     p.Stock,
		Category:  p.Category,
		Status:    gen.ProductStatus(p.Status),
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
	if p.Description != nil {
		d := *p.Description
		pr.Description = &d
	}
	c.JSON(http.StatusCreated, pr)
}

func (h *Handler) GetProduct(c *gin.Context, id openapi_types.UUID) {
	p, err := h.repo.GetByID(uuid.UUID(id))
	if err != nil {
		util.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error", nil)
		return
	}
	if p == nil {
		util.Error(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "product not found", nil)
		return
	}
	pr := gen.ProductResponse{
		Id:        openapi_types.UUID(p.ID),
		Name:      p.Name,
		Price:     p.Price,
		Stock:     p.Stock,
		Category:  p.Category,
		Status:    gen.ProductStatus(p.Status),
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
	if p.Description != nil {
		d := *p.Description
		pr.Description = &d
	}
	c.JSON(http.StatusOK, pr)
}

func (h *Handler) UpdateProduct(c *gin.Context, id openapi_types.UUID) {
	var req gen.ProductUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "ошибка валидации входных данных", nil)
		return
	}
	if v := util.ValidateProductUpdate(req); len(v) > 0 {
		util.ValidationDetails(c, v)
		return
	}
	p, err := h.repo.GetByID(uuid.UUID(id))
	if err != nil {
		util.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error", nil)
		return
	}
	if p == nil {
		util.Error(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "product not found", nil)
		return
	}
	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Description != nil {
		p.Description = req.Description
	}
	if req.Price != nil {
		p.Price = *req.Price
	}
	if req.Stock != nil {
		p.Stock = *req.Stock
	}
	if req.Category != nil {
		p.Category = *req.Category
	}
	if req.Status != nil {
		p.Status = string(*req.Status)
	}
	if err := h.repo.Update(uuid.UUID(id), p); err != nil {
		util.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "validation error", nil)
		return
	}
	np, _ := h.repo.GetByID(uuid.UUID(id))
	if np == nil {
		util.Error(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "product not found", nil)
		return
	}
	pr := gen.ProductResponse{
		Id:        openapi_types.UUID(np.ID),
		Name:      np.Name,
		Price:     np.Price,
		Stock:     np.Stock,
		Category:  np.Category,
		Status:    gen.ProductStatus(np.Status),
		CreatedAt: np.CreatedAt,
		UpdatedAt: np.UpdatedAt,
	}
	if np.Description != nil {
		d := *np.Description
		pr.Description = &d
	}
	c.JSON(http.StatusOK, pr)
}

func (h *Handler) DeleteProduct(c *gin.Context, id openapi_types.UUID) {
	p, err := h.repo.GetByID(uuid.UUID(id))
	if err != nil {
		util.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error", nil)
		return
	}
	if p == nil {
		util.Error(c, http.StatusNotFound, "PRODUCT_NOT_FOUND", "product not found", nil)
		return
	}
	if err := h.repo.Archive(uuid.UUID(id)); err != nil {
		util.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error", nil)
		return
	}
	c.Status(http.StatusNoContent)
}
