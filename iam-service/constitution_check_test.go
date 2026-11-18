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
	"golang.org/x/crypto/bcrypt"
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
	if err := model.AutoMigrateModels(db); err != nil {
		t.Fatal(err)
	}
	if err := model.Seed(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	// 多应用裁决 #8：权限/菜单管理接口需要应用上下文，冒烟夹具同步补应用
	app := model.Application{AppId: "900", Code: "test", Name: "Test App", Status: model.StatusEnabled, Version: 1}
	if err := db.Select(model.ApplicationFields).Create(&app).Error; err != nil {
		t.Fatal(err)
	}
	// 用户种子（bcrypt 密码摘要）；数据边界 ADR 0006：用户须有所属应用
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{Username: "test-user", Nickname: "Test User", PasswordHash: string(hash), Status: model.StatusEnabled, Version: 1, HomeApplicationId: app.Id}
	if err := db.Select(model.UserFields).Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	role := model.Role{Code: "test-role", Name: "Test Role", Status: model.StatusEnabled, ApplicationId: app.Id, Version: 1}
	if err := db.Select(model.RoleFields).Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	permission := model.Permission{Code: "test.perm.one", Name: "Test Permission", ApplicationId: app.Id, Type: "api", Status: model.StatusEnabled, Version: 1}
	if err := db.Select(model.PermissionFields).Create(&permission).Error; err != nil {
		t.Fatal(err)
	}

	capture := &sqlCapture{}
	db.Logger = capture
	ctx := auth.WithApplicationId(auth.WithAccountId(context.Background(), user.Id), app.Id)
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
	if _, err := service.NewAccountService(db, redisClient).GetAccount(ctx, &commonv1.GetRequest{Id: user.Id}); err != nil {
		t.Fatal(err)
	}
	if _, authErr := service.NewAuthenticationService(db, redisClient).PasswordAuth(ctx, &iamv1.PasswordAuthRequest{Identifier: user.Username, Password: "invalid"}); authErr != nil {
		if !errors.Is(authErr, service.ErrInvalidAccountPassword) {
			t.Fatal(authErr)
		}
	}
	loader := service.NewPermissionSnapshotLoader(db)
	if _, err := loader(ctx, user.Id); err != nil {
		t.Fatal(err)
	}

	for _, query := range capture.queries {
		if strings.Contains(query, "SELECT *") {
			t.Fatalf("SELECT * detected: %s", query)
		}
	}
}
