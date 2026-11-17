package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	common_v1 "github.com/linjunhao1997/idl/pb/common"
	sender_v1 "github.com/linjunhao1997/idl/pb/sender"

	"sender-service/pkg/model"
)

func TestCreateAndGetTemplate(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewTemplateService(db, testApplicationId)

	created, err := svc.CreateTemplate(operatorCtx(11), &sender_v1.CreateTemplateRequest{
		Code: "welcome", Name: "欢迎", Content: "hi {name}", Remark: "r",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Code != "welcome" || created.Name != "欢迎" || created.Version != 1 {
		t.Fatalf("created mismatch: %+v", created)
	}
	if created.CreatedBy != "11" || created.UpdatedBy != "11" {
		t.Fatalf("audit mismatch: %+v", created)
	}

	got, err := svc.GetTemplate(context.Background(), &common_v1.GetRequest{Id: mustParseID(created.Id)})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Content != "hi {name}" {
		t.Fatalf("content = %q", got.Content)
	}
}

func TestCreateTemplateDuplicatedCode(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewTemplateService(db, testApplicationId)

	req := &sender_v1.CreateTemplateRequest{Code: "dup", Name: "n", Content: "c"}
	if _, err := svc.CreateTemplate(operatorCtx(1), req); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := svc.CreateTemplate(operatorCtx(1), req); !errors.Is(err, ErrTemplateCodeDuplicated) {
		t.Fatalf("second create err = %v, want ErrTemplateCodeDuplicated", err)
	}

	// 跨应用同 code 允许
	other := NewTemplateService(db, 999)
	if _, err := other.CreateTemplate(operatorCtx(1), req); err != nil {
		t.Fatalf("cross-app create with same code: %v", err)
	}
}

func TestUpdateTemplateVersionAndFields(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewTemplateService(db, testApplicationId)

	created, err := svc.CreateTemplate(operatorCtx(3), &sender_v1.CreateTemplateRequest{Code: "c", Name: "old", Content: "old"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := mustParseID(created.Id)

	// 陈旧版本拒绝
	if _, err := svc.UpdateTemplate(operatorCtx(3), &sender_v1.UpdateTemplateRequest{
		Id: id, Name: "new", Version: 99,
	}); !errors.Is(err, ErrUpdateTemplateBusy) {
		t.Fatalf("stale version err = %v, want ErrUpdateTemplateBusy", err)
	}

	if _, err := svc.UpdateTemplate(operatorCtx(4), &sender_v1.UpdateTemplateRequest{
		Id: id, Name: "new", Content: "new-content", Version: created.Version,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := svc.GetTemplate(context.Background(), &common_v1.GetRequest{Id: id})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "new" || got.Content != "new-content" || got.UpdatedBy != "4" || got.Version != created.Version+1 {
		t.Fatalf("updated mismatch: %+v", got)
	}
}

func TestDeleteTemplateSoftDeletesAndFreesCode(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewTemplateService(db, testApplicationId)

	created, err := svc.CreateTemplate(operatorCtx(5), &sender_v1.CreateTemplateRequest{Code: "free", Name: "n", Content: "c"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := mustParseID(created.Id)
	if _, err := svc.DeleteTemplate(operatorCtx(5), &common_v1.GetRequest{Id: id}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var raw model.MessageTemplate
	if err := db.Where("id = ?", id).First(&raw).Error; err != nil {
		t.Fatalf("raw query: %v", err)
	}
	if !raw.IsDeleted || !strings.HasPrefix(raw.Code, "free(已删除_") {
		t.Fatalf("soft delete mismatch: is_deleted=%v code=%q", raw.IsDeleted, raw.Code)
	}
	// 同 code 可再次创建
	if _, err := svc.CreateTemplate(operatorCtx(5), &sender_v1.CreateTemplateRequest{Code: "free", Name: "n2", Content: "c2"}); err != nil {
		t.Fatalf("recreate after soft delete: %v", err)
	}
	// 删除后 get 按不存在
	if _, err := svc.GetTemplate(context.Background(), &common_v1.GetRequest{Id: id}); !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("get deleted err = %v, want ErrTemplateNotFound", err)
	}
}

func TestTemplateCrossAppInvisible(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewTemplateService(db, testApplicationId)
	created, err := svc.CreateTemplate(operatorCtx(1), &sender_v1.CreateTemplateRequest{Code: "secret", Name: "n", Content: "c"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	other := NewTemplateService(db, 999)
	if _, err := other.GetTemplate(context.Background(), &common_v1.GetRequest{Id: mustParseID(created.Id)}); !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("cross-app get err = %v, want ErrTemplateNotFound", err)
	}
}
