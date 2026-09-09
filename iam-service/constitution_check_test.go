package main_test

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	commonv1 "github.com/linjunhao1997/idl/pb/common"
	iamv1 "github.com/linjunhao1997/idl/pb/iam"
	"iam-service/pkg/auth"
	"iam-service/pkg/model"
	"iam-service/pkg/service"
)

type sqlCapture struct {
	logger.Interface
	queries []string
}

func (capture *sqlCapture) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	capture.queries = append(capture.queries, strings.ToUpper(sql))
}

func TestConstitutionCheck(t *testing.T) {
	command := exec.Command("python", "../scripts/check_constitution.py", ".")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("constitution scan failed:\n%s", output)
	}
}

func TestServiceQueriesDoNotUseSelectStar(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(model.Models...); err != nil {
		t.Fatal(err)
	}
	if err := model.Seed(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	account := model.Account{Code: "test-user", Name: "Test User", Email: "test@example.com", Status: "enabled", Kind: model.AccountKindIdUser}
	if err := db.Select(model.AccountFields).Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	var role model.Role
	if err := db.Select(model.RoleFields).First(&role).Error; err != nil {
		t.Fatal(err)
	}
	var permission model.Permission
	if err := db.Select(model.PermissionFields).First(&permission).Error; err != nil {
		t.Fatal(err)
	}

	capture := &sqlCapture{}
	db.Logger = capture
	ctx := auth.WithAccountId(context.Background(), account.Id)
	services := service.NewAuthorizationService(db, redisClient)
	if _, err := services.GetRole(ctx, &commonv1.GetRequest{Id: role.Id}); err != nil {
		t.Fatal(err)
	}
	if _, err := services.GetPermission(ctx, &commonv1.GetRequest{Id: permission.Id}); err != nil {
		t.Fatal(err)
	}
	if _, err := services.GetMyPermissions(ctx, &emptypb.Empty{}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.NewAccountService(db, redisClient).GetAccount(ctx, &commonv1.GetRequest{Id: account.Id}); err != nil {
		t.Fatal(err)
	}
	if _, authErr := service.NewAuthenticationService(db, redisClient).PasswordAuth(ctx, &iamv1.PasswordAuthRequest{Email: account.Email, Password: "invalid"}); authErr != nil {
		if !errors.Is(authErr, service.ErrInvalidAccountPassword) {
			t.Fatal(authErr)
		}
	}
	loader := service.NewPermissionSnapshotLoader(db)
	if _, err := loader(ctx, account.Id); err != nil {
		t.Fatal(err)
	}

	// Test resource queries don't use SELECT *
	capture.queries = nil
	app := model.Application{Code: "test-app", Name: "Test App"}
	if err := db.Select(model.ApplicationFields).Create(&app).Error; err != nil {
		t.Fatal(err)
	}
	resource := model.Resource{ApplicationId: app.Id, Kind: "menu", Key: "test.menu", Name: "Test Menu"}
	if err := db.Select(model.ResourceFields).Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := services.GetResource(ctx, &commonv1.GetRequest{Id: resource.Id}); err != nil {
		t.Fatal(err)
	}
	if _, err := services.ListResourceTree(ctx, &iamv1.ListResourceTreeRequest{ApplicationId: app.Id}); err != nil {
		t.Fatal(err)
	}

	for _, query := range capture.queries {
		if strings.Contains(query, "SELECT *") {
			t.Fatalf("SELECT * detected: %s", query)
		}
	}
}
