package service

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	sender_v1 "github.com/linjunhao1997/idl/pb/sender"
	"github.com/linjunhao1997/contrib/pkg/authz"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"sender-service/pkg/model"
)

const testApplicationId int64 = 2

func setupMessageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := model.AutoMigrateModels(db); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

// operatorCtx 注入审计身份（authz 身份仅审计用；判权由拦截器保证，测试直接注入）。
func operatorCtx(accountId int64) context.Context {
	return authz.WithAccountId(context.Background(), accountId)
}

func TestSendMessageCreatesSentRecord(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db, testApplicationId)

	resp, err := svc.SendMessage(operatorCtx(42), &sender_v1.SendMessageRequest{
		Recipient:    "user@example.com",
		TemplateCode: "welcome",
		Content:      "hello",
		Channel:      "email",
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if resp.SendStatus != model.SendStatusSent {
		t.Fatalf("send_status = %q, want %q", resp.SendStatus, model.SendStatusSent)
	}
	if resp.CreatedBy != "42" || resp.UpdatedBy != "42" {
		t.Fatalf("audit fields = (%q,%q), want 42/42", resp.CreatedBy, resp.UpdatedBy)
	}

	var got model.MessageRecord
	if err := db.First(&got, "id = ?", mustParseID(resp.Id)).Error; err != nil {
		t.Fatalf("query record: %v", err)
	}
	if got.SendStatus != model.SendStatusSent {
		t.Fatalf("persisted send_status = %q, want sent", got.SendStatus)
	}
	if got.ApplicationId != testApplicationId {
		t.Fatalf("application_id = %d, want %d", got.ApplicationId, testApplicationId)
	}
	if got.Recipient != "user@example.com" || got.Channel != "email" {
		t.Fatalf("record fields mismatch: recipient=%q channel=%q", got.Recipient, got.Channel)
	}
}

func TestGetMessageScopedToApplicationAndNotDeleted(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db, testApplicationId)

	resp, err := svc.SendMessage(operatorCtx(7), &sender_v1.SendMessageRequest{Recipient: "a", Content: "c"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	id := mustParseID(resp.Id)

	// 同应用能取到
	if _, err := svc.GetMessage(context.Background(), &common_v1.GetRequest{Id: id}); err != nil {
		t.Fatalf("get own record: %v", err)
	}
	// 跨应用查询按不存在处理
	otherApp := NewMessageService(db, 999)
	if _, err := otherApp.GetMessage(context.Background(), &common_v1.GetRequest{Id: id}); !errors.Is(err, ErrMessageNotFound) {
		t.Fatalf("cross-app get err = %v, want ErrMessageNotFound", err)
	}

	// 软删除后查询按不存在处理
	if _, err := svc.DeleteMessage(operatorCtx(7), &common_v1.GetRequest{Id: id}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.GetMessage(context.Background(), &common_v1.GetRequest{Id: id}); !errors.Is(err, ErrMessageNotFound) {
		t.Fatalf("get deleted err = %v, want ErrMessageNotFound", err)
	}
}

func TestDeleteMessageSoftDeletesWithAudit(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db, testApplicationId)

	resp, err := svc.SendMessage(operatorCtx(9), &sender_v1.SendMessageRequest{Recipient: "b", Content: "c"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	id := mustParseID(resp.Id)
	if _, err := svc.DeleteMessage(operatorCtx(9), &common_v1.GetRequest{Id: id}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var got model.MessageRecord
	// Unscoped 不可用（未启用 gorm.DeletedAt），直接按 id 查行
	if err := db.Where("id = ?", id).First(&got).Error; err != nil {
		t.Fatalf("query raw row: %v", err)
	}
	if !got.IsDeleted {
		t.Fatal("is_deleted not set")
	}
	if got.UpdatedBy != "9" {
		t.Fatalf("updated_by = %q, want 9", got.UpdatedBy)
	}
}

func TestListMessagesFiltersByAppAndStatus(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db, testApplicationId)

	for i := 0; i < 3; i++ {
		if _, err := svc.SendMessage(operatorCtx(1), &sender_v1.SendMessageRequest{
			Recipient: "list@example.com", Content: "c", Channel: "email",
		}); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	// 插入一条跨应用记录
	cross := model.MessageRecord{Id: 123456, ApplicationId: 999, Recipient: "x", SendStatus: model.SendStatusSent, Version: 1}
	if err := db.Create(&cross).Error; err != nil {
		t.Fatalf("seed cross-app: %v", err)
	}

	resp, err := svc.ListMessages(context.Background(), &sender_v1.ListMessagesRequest{
		CurrentPage: 1, PageSize: 10, SendStatus: model.SendStatusSent,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if resp.Page.Total != 3 {
		t.Fatalf("total = %d, want 3 (cross-app excluded)", resp.Page.Total)
	}
	if len(resp.Records) != 3 {
		t.Fatalf("records = %d, want 3", len(resp.Records))
	}
}

func mustParseID(s string) int64 {
	return cast.ToInt64(s)
}
