package main_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	common_v1 "github.com/linjunhao1997/idl/pb/common"
	sender_v1 "github.com/linjunhao1997/idl/pb/sender"
	"github.com/spf13/cast"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"sender-service/pkg/model"
	"sender-service/pkg/service"
)

type sqlCapture struct {
	logger.Interface
	queries []string
}

func (c *sqlCapture) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	c.queries = append(c.queries, strings.ToUpper(sql))
}

func TestConstitutionCheck(t *testing.T) {
	cmd := exec.Command("python", "../scripts/check_constitution.py", ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("constitution scan failed:\n%s", out)
	}
}

func TestServiceQueriesDoNotUseSelectStar(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrateModels(db); err != nil {
		t.Fatal(err)
	}
	capture := &sqlCapture{}
	db.Logger = capture

	msgSvc := service.NewMessageService(db, 2)
	tplSvc := service.NewTemplateService(db, 2)
	ctx := context.Background()

	rec, err := msgSvc.SendMessage(ctx, &sender_v1.SendMessageRequest{Recipient: "a", Content: "c"})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = msgSvc.GetMessage(ctx, &common_v1.GetRequest{Id: cast.ToInt64(rec.Id)})
	_, _ = msgSvc.ListMessages(ctx, &sender_v1.ListMessagesRequest{CurrentPage: 1, PageSize: 10})

	tpl, err := tplSvc.CreateTemplate(ctx, &sender_v1.CreateTemplateRequest{Code: "c", Name: "n", Content: "x"})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = tplSvc.GetTemplate(ctx, &common_v1.GetRequest{Id: cast.ToInt64(tpl.Id)})
	_, _ = tplSvc.ListTemplates(ctx, &sender_v1.ListTemplatesRequest{CurrentPage: 1, PageSize: 10})

	for _, q := range capture.queries {
		if strings.Contains(q, "SELECT *") {
			t.Fatalf("SELECT * detected: %s", q)
		}
	}
}
