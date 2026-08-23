package usecase

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
)

type ProductUsecase struct {
	productRepo    domain.ProductRepository
	companyRepo    domain.CompanyRepository
	catalogRepo    domain.CatalogRepository
	sinProductRepo domain.SinProductRepository
	docSectorRepo  domain.SiatActividadDocSectorRepository
}

func NewProductUsecase(productRepo domain.ProductRepository, companyRepo domain.CompanyRepository, catalogRepo domain.CatalogRepository, extras ...any) *ProductUsecase {
	uc := &ProductUsecase{productRepo: productRepo, companyRepo: companyRepo, catalogRepo: catalogRepo}
	for _, extra := range extras {
		switch typed := extra.(type) {
		case domain.SinProductRepository:
			uc.sinProductRepo = typed
		case domain.SiatActividadDocSectorRepository:
			uc.docSectorRepo = typed
		}
	}
	return uc
}

type ProductMappingRequest struct {
	CodigoProductoSin     int64  `json:"codigo_producto_sin"`
	CodigoActividad       string `json:"codigo_actividad"`
	CodigoDocumentoSector int    `json:"codigo_documento_sector"`
	UnidadMedida          int    `json:"unidad_medida"`
	IsDefault             bool   `json:"is_default"`
}

type CreateProductRequest struct {
	CompanyID string                  `json:"company_id"`
	SKU       string                  `json:"sku"`
	Name      string                  `json:"name"`
	Mappings  []ProductMappingRequest `json:"mappings"`
}

func (uc *ProductUsecase) Create(req CreateProductRequest) (*domain.Product, error) {
	if strings.TrimSpace(req.CompanyID) == "" || strings.TrimSpace(req.SKU) == "" || strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("company_id, sku y name son obligatorios")
	}
	if _, err := uc.companyRepo.GetByID(req.CompanyID); err != nil {
		return nil, errors.New("empresa no encontrada")
	}
	product := &domain.Product{CompanyID: req.CompanyID, SKU: strings.TrimSpace(req.SKU), Name: strings.TrimSpace(req.Name), Active: true}
	validatedMappings := make([]domain.ProductMapping, 0, len(req.Mappings))
	for _, input := range req.Mappings {
		mapping, err := uc.validateMapping(req.CompanyID, "", input)
		if err != nil {
			return nil, err
		}
		validatedMappings = append(validatedMappings, mapping)
	}
	if err := uc.productRepo.Create(product); err != nil {
		return nil, err
	}
	for _, mapping := range validatedMappings {
		mapping.ProductID = product.ID
		if err := uc.productRepo.UpsertMapping(req.CompanyID, mapping); err != nil {
			return nil, err
		}
		product.Mappings = append(product.Mappings, mapping)
	}
	return product, nil
}

func (uc *ProductUsecase) AddMapping(companyID, productID string, req ProductMappingRequest) error {
	mapping, err := uc.validateMapping(companyID, productID, req)
	if err != nil {
		return err
	}
	return uc.productRepo.UpsertMapping(companyID, mapping)
}

func (uc *ProductUsecase) List(companyID string) ([]*domain.Product, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("companyId es obligatorio")
	}
	return uc.productRepo.List(companyID)
}

func (uc *ProductUsecase) validateMapping(companyID, productID string, req ProductMappingRequest) (domain.ProductMapping, error) {
	if req.CodigoProductoSin <= 0 || strings.TrimSpace(req.CodigoActividad) == "" || req.CodigoDocumentoSector <= 0 || req.UnidadMedida <= 0 {
		return domain.ProductMapping{}, errors.New("codigo_producto_sin, codigo_actividad, codigo_documento_sector y unidad_medida son obligatorios")
	}
	var sinProduct *domain.SinProduct
	if uc.sinProductRepo != nil {
		var err error
		sinProduct, err = uc.sinProductRepo.GetByCode(companyID, req.CodigoProductoSin)
		if err != nil || sinProduct == nil {
			return domain.ProductMapping{}, fmt.Errorf("codigo_producto_sin %d no existe en el catálogo SIN sincronizado", req.CodigoProductoSin)
		}
	} else if !uc.catalogContains(companyID, "productosServicios", int(req.CodigoProductoSin)) {
		return domain.ProductMapping{}, fmt.Errorf("codigo_producto_sin %d no existe en el catálogo sincronizado", req.CodigoProductoSin)
	}
	if !uc.catalogContains(companyID, "actividades", parseInt(req.CodigoActividad)) {
		return domain.ProductMapping{}, fmt.Errorf("codigo_actividad %q no existe en el catálogo sincronizado", req.CodigoActividad)
	}
	if !uc.catalogContains(companyID, "unidadMedida", req.UnidadMedida) {
		return domain.ProductMapping{}, fmt.Errorf("unidad_medida %d no existe en el catálogo sincronizado", req.UnidadMedida)
	}
	if uc.docSectorRepo == nil {
		return domain.ProductMapping{}, errors.New("no se pudo consultar actividadesDocumentoSector; sincronice los catálogos")
	}
	sectorItems, err := uc.docSectorRepo.ListByActividad(companyID, strings.TrimSpace(req.CodigoActividad))
	if err != nil {
		return domain.ProductMapping{}, errors.New("no se pudo consultar actividadesDocumentoSector; sincronice los catálogos")
	}
	compatible := false
	for _, item := range sectorItems {
		if item.CodigoDocumentoSector == req.CodigoDocumentoSector {
			compatible = true
			break
		}
	}
	if !compatible {
		return domain.ProductMapping{}, fmt.Errorf("la actividad %s no está habilitada para el sector %d", req.CodigoActividad, req.CodigoDocumentoSector)
	}
	var sinProductID *string
	if sinProduct != nil {
		sinProductID = &sinProduct.ID
	}
	return domain.ProductMapping{ProductID: productID, SinProductID: sinProductID, CodigoProductoSin: req.CodigoProductoSin, CodigoActividad: strings.TrimSpace(req.CodigoActividad), CodigoDocumentoSector: req.CodigoDocumentoSector, UnidadMedida: req.UnidadMedida, IsDefault: req.IsDefault, Active: true, SyncedAt: time.Now()}, nil
}

func (uc *ProductUsecase) catalogContains(companyID, tipo string, code int) bool {
	if code <= 0 || uc.catalogRepo == nil {
		return false
	}
	items, err := uc.catalogRepo.List(companyID, tipo)
	if err != nil {
		return false
	}
	for _, item := range items {
		if item.Codigo == code {
			return true
		}
	}
	return false
}

func parseInt(value string) int {
	parsed, _ := strconv.Atoi(strings.TrimSpace(value))
	return parsed
}
