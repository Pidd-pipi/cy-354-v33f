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
	stored := *p
	f.products[p.ID] = &stored
	return nil
}

func (f *fakeProductRepo) FindByID(_ context.Context, id uint) (*model.Product, error) {
	if p, ok := f.products[id]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, util.ErrNotFound
}

func (f *fakeProductRepo) List(_ context.Context, category, campus, keyword, status string, page, pageSize int) ([]model.Product, int64, error) {
	var out []model.Product
	for _, p := range f.products {
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
	f.products[id].Version++
	return nil
}

func (f *fakeProductRepo) UpdateEditable(_ context.Context, p *model.Product, expectedVersion uint) error {
	cur, err := f.FindByID(context.Background(), p.ID)
	if err != nil {
		return err
	}
	if cur.Version != expectedVersion {
		return util.ErrConflict
	}
	stored := f.products[p.ID]
	stored.Title = p.Title
	stored.Description = p.Description
	stored.Price = p.Price
	stored.Condition = p.Condition
	stored.TradeLocation = p.TradeLocation
	stored.Version = expectedVersion + 1
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
	newSvc := func() (*ProductService, *model.Product) {
		repo := newFakeProductRepo()
		svc := NewProductService(repo, slog.Default())
		created, _ := svc.Create(context.Background(), 1, &dto.CreateProductRequest{Title: "我的书", Price: 10, Category: constants.ProductCategoryBooks, Condition: "全新", Campus: "东校区", TradeLocation: "东门"})
		return svc, created
	}
	editReq := func(version uint) *dto.UpdateProductRequest {
		return &dto.UpdateProductRequest{Title: "我的书（降价）", Description: "笔记很少", Price: 8, Condition: "八成新", TradeLocation: "西门", Version: version}
	}

	t.Run("owner edits with current version", func(t *testing.T) {
		svc, created := newSvc()
		updated, err := svc.Update(context.Background(), 1, created.ID, editReq(created.Version))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Price != 8 || updated.Title != "我的书（降价）" || updated.Condition != "八成新" || updated.TradeLocation != "西门" {
			t.Fatalf("editable fields not applied: %+v", updated)
		}
		if updated.Version != created.Version+1 {
			t.Fatalf("expected version bump to %d, got %d", created.Version+1, updated.Version)
		}
	})

	t.Run("stale version rejected and latest returned", func(t *testing.T) {
		svc, created := newSvc()
		if _, err := svc.Update(context.Background(), 1, created.ID, editReq(created.Version)); err != nil {
			t.Fatalf("first update failed: %v", err)
		}
		latest, err := svc.Update(context.Background(), 1, created.ID, editReq(created.Version))
		var conflict *EditConflictError
		if !errors.As(err, &conflict) {
			t.Fatalf("expected EditConflictError, got %v", err)
		}
		if latest == nil || latest.Version != created.Version+1 || latest.Price != 8 {
			t.Fatalf("expected latest row returned, got %+v", latest)
		}
	})

	t.Run("non-owner forbidden", func(t *testing.T) {
		svc, created := newSvc()
		if _, err := svc.Update(context.Background(), 99, created.ID, editReq(created.Version)); err == nil {
			t.Fatalf("expected forbidden error for non-owner")
		}
	})

	t.Run("sold or removed not editable", func(t *testing.T) {
		svc, created := newSvc()
		if _, err := svc.Remove(context.Background(), 1, created.ID); err != nil {
			t.Fatalf("remove failed: %v", err)
		}
		fresh, _ := svc.Get(context.Background(), created.ID)
		if _, err := svc.Update(context.Background(), 1, created.ID, editReq(fresh.Version)); err == nil {
			t.Fatalf("expected not-editable error for removed product")
		}
	})
}
