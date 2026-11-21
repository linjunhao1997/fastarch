package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/linjunhao1997/contrib/pkg/authz"
	home_v1 "github.com/linjunhao1997/idl/pb/home"
	"gorm.io/gorm"

	"home-service/pkg/model"
)

const testApplicationId int64 = 4

func setupHomepageTestDB(t *testing.T) *gorm.DB {
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

func operatorCtx(accountId int64) context.Context {
	return authz.WithAccountId(context.Background(), accountId)
}

func TestGetHomepageFallsBackToDefault(t *testing.T) {
	svc := NewHomepageService(setupHomepageTestDB(t), testApplicationId)
	got, err := svc.GetHomepage(context.Background(), &home_v1.GetHomepageRequest{})
	if err != nil {
		t.Fatalf("get homepage: %v", err)
	}
	if got.DraftJson != DefaultHomepageJSON {
		t.Fatalf("default draft mismatch")
	}
	if got.Version != 0 || got.Id != "0" {
		t.Fatalf("default meta = (%s,%d)", got.Id, got.Version)
	}
}

func TestSaveDraftPublishAndPublishedRead(t *testing.T) {
	db := setupHomepageTestDB(t)
	svc := NewHomepageService(db, testApplicationId)
	content := `{"brandName":"测试企业","hero":{"title":"新标题"}}`

	saved, err := svc.SaveDraft(operatorCtx(7), &home_v1.SaveDraftRequest{ContentJson: content})
	if err != nil {
		t.Fatalf("save draft: %v", err)
	}
	if saved.Version != 1 || saved.DraftJson != content || saved.UpdatedBy != "7" {
		t.Fatalf("saved = %+v", saved)
	}

	published, err := svc.PublishHomepage(operatorCtx(8), &home_v1.PublishHomepageRequest{ExpectedVersion: 1})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.PublishedJson != content || published.PublishedAt == nil || published.Version != 2 {
		t.Fatalf("published = %+v", published)
	}

	got, err := svc.GetPublishedHomepage(context.Background(), &home_v1.GetPublishedHomepageRequest{})
	if err != nil {
		t.Fatalf("get published: %v", err)
	}
	if got.ContentJson != content || got.PublishedAt == nil {
		t.Fatalf("published read = %+v", got)
	}

	// 再写草稿不影响已发布
	next := `{"brandName":"草稿企业"}`
	if _, err := svc.SaveDraft(operatorCtx(9), &home_v1.SaveDraftRequest{ContentJson: next, ExpectedVersion: 2}); err != nil {
		t.Fatalf("second save: %v", err)
	}
	got, err = svc.GetPublishedHomepage(context.Background(), &home_v1.GetPublishedHomepageRequest{})
	if err != nil {
		t.Fatalf("get published after draft: %v", err)
	}
	if got.ContentJson != content {
		t.Fatalf("published changed by draft: %q", got.ContentJson)
	}
}

func TestSaveDraftVersionConflict(t *testing.T) {
	svc := NewHomepageService(setupHomepageTestDB(t), testApplicationId)
	content := `{"brandName":"A"}`
	if _, err := svc.SaveDraft(operatorCtx(1), &home_v1.SaveDraftRequest{ContentJson: content}); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if _, err := svc.SaveDraft(operatorCtx(1), &home_v1.SaveDraftRequest{ContentJson: content, ExpectedVersion: 1}); err != nil {
		t.Fatalf("second save: %v", err)
	}
	if _, err := svc.SaveDraft(operatorCtx(1), &home_v1.SaveDraftRequest{ContentJson: content, ExpectedVersion: 1}); !errors.Is(err, ErrHomepageVersionConflict) {
		t.Fatalf("stale save err = %v, want conflict", err)
	}
}

func TestValidateHomepageJSON(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    error
	}{
		{"empty", "  ", ErrInvalidHomepageJson},
		{"array", `[1,2]`, ErrInvalidHomepageJson},
		{"bad", `{"a":`, ErrInvalidHomepageJson},
		{"ok", `{"brandName":"A"}`, nil},
	}
	for _, c := range cases {
		err := validateHomepageJSON(c.content)
		if !errors.Is(err, c.want) {
			t.Fatalf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}

	if err := validateHomepageJSON(strings.Repeat("x", maxHomepageContentBytes+1)); !errors.Is(err, ErrHomepageTooLarge) {
		t.Fatalf("too large err = %v", err)
	}
}
