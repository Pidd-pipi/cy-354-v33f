package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/lp/campus-market/internal/constants"
	"github.com/lp/campus-market/internal/dto"
	"github.com/lp/campus-market/internal/model"
	"github.com/lp/campus-market/internal/util"
)

type fakeProductRepo struct {
	products map[uint]*model.Product
	nextID   uint
}

func newFakeProductRepo() *fakeProductRepo {
	return &fakeProductRepo{products: map[uint]*model.Product{}, nextID: 1}
}

func (f *fakeProductRepo) Create(_ context.Context, p *model.Product) error {
	p.ID = f.nextID
	f.nextID++
	f.products[p.ID] = p
	return nil
}

func (f *fakeProductRepo) FindByID(_ context.Context, id uint) (*model.Product, error) {
	if p, ok := f.products[id]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, util.ErrNotFound
}

func (f *fakeProductRepo) List(_ context.Context, sellerID uint, category, campus, keyword, status string, page, pageSize int) ([]model.Product, int64, error) {
	var out []model.Product
	for _, p := range f.products {
		if sellerID != 0 && p.SellerID != sellerID {
			continue
		}
		if category != "" && p.Category != category {
			continue
		}
		if campus != "" && p.Campus != campus {
			continue
		}
		if status != "" && p.Status != status {
			continue
		}
		out = append(out, *p)
	}
	return out, int64(len(out)), nil
}

func (f *fakeProductRepo) UpdateStatus(_ context.Context, id uint, status string) error {
	_, err := f.FindByID(context.Background(), id)
	if err != nil {
		return err
	}
	f.products[id].Status = status
	return nil
}

// UpdateWithRevision emulates the optimistic-lock UPDATE of the GORM repo.
func (f *fakeProductRepo) UpdateWithRevision(_ context.Context, id, expectedRevision uint, fields map[string]interface{}) error {
	p, ok := f.products[id]
	if !ok {
		return util.ErrNotFound
	}
	if p.Revision != expectedRevision || p.Status != constants.ProductStatusOnSale {
		return util.ErrConflict
	}
	if v, ok := fields["title"].(string); ok {
		p.Title = v
	}
	if v, ok := fields["description"].(string); ok {
		p.Description = v
	}
	if v, ok := fields["price"].(float64); ok {
		p.Price = v
	}
	if v, ok := fields["condition"].(string); ok {
		p.Condition = v
	}
	if v, ok := fields["trade_location"].(string); ok {
		p.TradeLocation = v
	}
	p.Revision++
	return nil
}

func (f *fakeProductRepo) Count(context.Context) (int64, error) { return int64(len(f.products)), nil }

func TestProductServiceCreate(t *testing.T) {
	svc := NewProductService(newFakeProductRepo(), slog.Default())
	tests := []struct {
		name     string
		category string
		wantErr  bool
	}{
		{name: "valid books", category: constants.ProductCategoryBooks, wantErr: false},
		{name: "valid electronics", category: constants.ProductCategoryElectronics, wantErr: false},
		{name: "invalid category", category: "sports", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &dto.CreateProductRequest{Title: "测试商品", Price: 10, Category: tt.category, Condition: "全新", Campus: "东校区", TradeLocation: "东门"}
			_, err := svc.Create(context.Background(), 1, req)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestProductServiceRemoveOwnership(t *testing.T) {
	repo := newFakeProductRepo()
	svc := NewProductService(repo, slog.Default())
	created, _ := svc.Create(context.Background(), 1, &dto.CreateProductRequest{Title: "我的书", Price: 10, Category: constants.ProductCategoryBooks, Condition: "全新", Campus: "东校区", TradeLocation: "东门"})
	if _, err := svc.Remove(context.Background(), 99, created.ID); err == nil {
		t.Fatalf("expected forbidden error for non-owner")
	}
	removed, err := svc.Remove(context.Background(), 1, created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed.Status != constants.ProductStatusRemoved {
		t.Fatalf("expected removed status")
	}
}

func TestProductServiceUpdate(t *testing.T) {
	repo := newFakeProductRepo()
	svc := NewProductService(repo, slog.Default())
	created, _ := svc.Create(context.Background(), 1, &dto.CreateProductRequest{Title: "旧标题", Price: 10, Category: constants.ProductCategoryBooks, Condition: "全新", Campus: "东校区", TradeLocation: "东门"})

	t.Run("non-owner forbidden", func(t *testing.T) {
		req := &dto.UpdateProductRequest{Title: "新标题", Price: 20, Condition: "九成新", TradeLocation: "西门", Revision: 0}
		if _, err := svc.Update(context.Background(), 99, created.ID, req); err == nil {
			t.Fatalf("expected forbidden error for non-owner")
		}
	})

	t.Run("first edit applies and bumps revision", func(t *testing.T) {
		req := &dto.UpdateProductRequest{Title: "新标题", Description: "新说明", Price: 20, Condition: "九成新", TradeLocation: "西门", Revision: 0}
		updated, err := svc.Update(context.Background(), 1, created.ID, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Title != "新标题" || updated.Price != 20 || updated.Revision != 1 {
			t.Fatalf("edit not applied: %+v", updated)
		}
	})

	t.Run("stale revision rejected and newest returned", func(t *testing.T) {
		req := &dto.UpdateProductRequest{Title: "覆盖别人", Price: 30, Condition: "全新", TradeLocation: "南门", Revision: 0}
		_, err := svc.Update(context.Background(), 1, created.ID, req)
		if err == nil {
			t.Fatalf("expected conflict error for stale revision")
		}
		var appErr *util.AppError
		if !errors.As(err, &appErr) || appErr.Data == nil {
			t.Fatalf("expected AppError carrying latest product, got %v", err)
		}
		latest := appErr.Data.(*model.Product)
		if latest.Revision != 1 || latest.Title != "新标题" {
			t.Fatalf("expected newest version in data, got %+v", latest)
		}
	})

	t.Run("edit closed after take-down", func(t *testing.T) {
		if _, err := svc.Remove(context.Background(), 1, created.ID); err != nil {
			t.Fatalf("remove failed: %v", err)
		}
		req := &dto.UpdateProductRequest{Title: "还想改", Price: 5, Condition: "全新", TradeLocation: "北门", Revision: 1}
		if _, err := svc.Update(context.Background(), 1, created.ID, req); err == nil {
			t.Fatalf("expected conflict error for removed product")
		}
	})
}
