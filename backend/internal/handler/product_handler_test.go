package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lp/campus-market/internal/constants"
	"github.com/lp/campus-market/internal/middleware"
	"github.com/lp/campus-market/internal/model"
	"github.com/lp/campus-market/internal/service"
	"github.com/lp/campus-market/internal/util"
)

// fakeProductRepo is an in-memory service.ProductRepository for handler tests.
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

func (f *fakeProductRepo) List(context.Context, string, string, string, string, int, int) ([]model.Product, int64, error) {
	return nil, 0, nil
}

func (f *fakeProductRepo) UpdateStatus(_ context.Context, id uint, status string) error {
	p, ok := f.products[id]
	if !ok {
		return util.ErrNotFound
	}
	p.Status = status
	p.Version++
	return nil
}

func (f *fakeProductRepo) UpdateEditable(_ context.Context, p *model.Product, expectedVersion uint) error {
	stored, ok := f.products[p.ID]
	if !ok {
		return util.ErrNotFound
	}
	if stored.Version != expectedVersion {
		return util.ErrConflict
	}
	stored.Title = p.Title
	stored.Description = p.Description
	stored.Price = p.Price
	stored.Condition = p.Condition
	stored.TradeLocation = p.TradeLocation
	stored.Version = expectedVersion + 1
	return nil
}

func (f *fakeProductRepo) Count(context.Context) (int64, error) { return int64(len(f.products)), nil }

// setupProductRouter mounts the update endpoint behind a stub that injects the
// authenticated user id, mirroring middleware.AuthRequired.
func setupProductRouter(svc *service.ProductService, userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	h := NewProductHandler(svc, slog.Default())
	r.PUT("/products/:id", h.Update)
	return r
}

func putProduct(t *testing.T, r *gin.Engine, id uint, body string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/products/"+strconv.Itoa(int(id)), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not json: %v", err)
	}
	return w.Code, resp
}

func TestProductHandlerUpdateConflictReturnsLatest(t *testing.T) {
	repo := newFakeProductRepo()
	svc := service.NewProductService(repo, slog.Default())
	seeded := &model.Product{
		SellerID: 7, Title: "旧书", Price: 10, Category: constants.ProductCategoryBooks,
		Condition: "九成新", Campus: "东校区", TradeLocation: "东门",
		Status: constants.ProductStatusOnSale, Version: 1,
	}
	if err := repo.Create(context.Background(), seeded); err != nil {
		t.Fatalf("seed: %v", err)
	}
	router := setupProductRouter(svc, 7)

	// First save with the current revision succeeds and bumps the version.
	code, resp := putProduct(t, router, seeded.ID, `{"title":"旧书·改","description":"d","price":9,"condition":"八成新","trade_location":"西门","version":1}`)
	if code != http.StatusOK {
		t.Fatalf("first update: expected 200, got %d (%v)", code, resp)
	}
	data := resp["data"].(map[string]interface{})
	if data["version"].(float64) != 2 {
		t.Fatalf("expected version 2 after save, got %v", data["version"])
	}

	// A second save replaying the stale revision is rejected and the response
	// carries the latest row so the client never overwrites the first edit.
	code, resp = putProduct(t, router, seeded.ID, `{"title":"覆盖标题","description":"x","price":1,"condition":"七成新","trade_location":"北门","version":1}`)
	if code != http.StatusConflict {
		t.Fatalf("stale update: expected 409, got %d (%v)", code, resp)
	}
	if resp["code"].(float64) != float64(constants.CodeConflict) {
		t.Fatalf("expected conflict code, got %v", resp["code"])
	}
	latest := resp["data"].(map[string]interface{})
	if latest["title"].(string) != "旧书·改" || latest["version"].(float64) != 2 {
		t.Fatalf("expected latest row in conflict payload, got %v", latest)
	}
	if repo.products[seeded.ID].Title != "旧书·改" {
		t.Fatalf("stale save must not overwrite stored row, got %q", repo.products[seeded.ID].Title)
	}
}

func TestProductHandlerUpdateNotEditableWhenSold(t *testing.T) {
	repo := newFakeProductRepo()
	svc := service.NewProductService(repo, slog.Default())
	seeded := &model.Product{
		SellerID: 7, Title: "已售出的书", Price: 10, Category: constants.ProductCategoryBooks,
		Condition: "九成新", Campus: "东校区", TradeLocation: "东门",
		Status: constants.ProductStatusSold, Version: 3,
	}
	if err := repo.Create(context.Background(), seeded); err != nil {
		t.Fatalf("seed: %v", err)
	}
	router := setupProductRouter(svc, 7)

	code, resp := putProduct(t, router, seeded.ID, `{"title":"试图改价","description":"x","price":1,"condition":"七成新","trade_location":"北门","version":3}`)
	if code != http.StatusConflict {
		t.Fatalf("sold product update: expected 409, got %d (%v)", code, resp)
	}
	if resp["message"].(string) != constants.MsgProductNotEditable {
		t.Fatalf("expected not-editable message, got %v", resp["message"])
	}
}

func TestProductHandlerUpdateForbiddenForNonOwner(t *testing.T) {
	repo := newFakeProductRepo()
	svc := service.NewProductService(repo, slog.Default())
	seeded := &model.Product{
		SellerID: 7, Title: "别人的书", Price: 10, Category: constants.ProductCategoryBooks,
		Condition: "九成新", Campus: "东校区", TradeLocation: "东门",
		Status: constants.ProductStatusOnSale, Version: 1,
	}
	if err := repo.Create(context.Background(), seeded); err != nil {
		t.Fatalf("seed: %v", err)
	}
	router := setupProductRouter(svc, 99)

	code, _ := putProduct(t, router, seeded.ID, `{"title":"越权改","description":"x","price":1,"condition":"七成新","trade_location":"北门","version":1}`)
	if code != http.StatusForbidden {
		t.Fatalf("non-owner update: expected 403, got %d", code)
	}
}
