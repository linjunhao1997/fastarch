package service

import (
	"context"
	"testing"

	common_v1 "github.com/linjunhao1997/idl/pb/common"
	common_dict_v1 "github.com/linjunhao1997/idl/pb/common/dict"
	"github.com/go-dev-frame/sponge/pkg/krand"
	"gorm.io/gorm"

	contribmodel "github.com/linjunhao1997/contrib/model"
	"sender-service/pkg/model"
)

func seedMenu(t *testing.T, db *gorm.DB, appId int64, name, code string) int64 {
	t.Helper()
	id := krand.NewID()
	m := contribmodel.Menu{
		Id: id, ApplicationId: appId, Name: name, Path: "/" + name,
		PermissionCode: code, Status: model.StatusEnabled, Version: 1,
		CreatedBy: "system", UpdatedBy: "system",
	}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("seed menu: %v", err)
	}
	return id
}

func TestMenuServiceFixedAppIsolation(t *testing.T) {
	db := setupMessageTestDB(t)
	// 一条属于 sender(appId=2)，一条属于别的应用
	ownId := seedMenu(t, db, testApplicationId, "own", "sender.record.list")
	otherId := seedMenu(t, db, 999, "other", "sender.record.list")

	svc, err := NewMenuService(db, testApplicationId)
	if err != nil {
		t.Fatalf("new menu service: %v", err)
	}

	list, err := svc.ListMenuTree(context.Background(), nil)
	if err != nil {
		t.Fatalf("list menu tree: %v", err)
	}
	if len(list.Roots) != 1 || mustParseID(list.Roots[0].Menu.Id) != ownId {
		t.Fatalf("menu tree = %+v, want only sender's menu", list.Roots)
	}

	// 跨应用菜单按不存在处理
	if _, err := svc.GetMenu(context.Background(), &common_v1.GetRequest{Id: otherId}); err == nil {
		t.Fatal("cross-app GetMenu should fail")
	}
}

func TestMenuPermissionCodeValidation(t *testing.T) {
	cases := []struct {
		code    string
		wantErr bool
	}{
		{"sender.record.send", false},
		{"iam.user.create", true},       // 服务段不符
		{"sender.record", true},         // 不足三段
		{"Sender.record.send", true},    // 大写非法
		{"sender.record..send", true},   // 空段
	}
	for _, tc := range cases {
		err := validateSenderPermissionCode(context.Background(), testApplicationId, tc.code)
		if (err != nil) != tc.wantErr {
			t.Fatalf("validate %q: err=%v wantErr=%v", tc.code, err, tc.wantErr)
		}
	}
}

func TestDictServiceFixedAppIsolation(t *testing.T) {
	db := setupMessageTestDB(t)
	// sender 应用下一个字典类型，别的应用下一个
	selfType := contribmodel.DictType{Id: krand.NewID(), ApplicationId: testApplicationId, Code: "channel", Name: "渠道", Version: 1, CreatedBy: "system", UpdatedBy: "system"}
	otherType := contribmodel.DictType{Id: krand.NewID(), ApplicationId: 999, Code: "channel", Name: "渠道", Version: 1, CreatedBy: "system", UpdatedBy: "system"}
	if err := db.Create(&selfType).Error; err != nil {
		t.Fatalf("seed self dict type: %v", err)
	}
	if err := db.Create(&otherType).Error; err != nil {
		t.Fatalf("seed other dict type: %v", err)
	}

	svc, err := NewDictService(db, testApplicationId)
	if err != nil {
		t.Fatalf("new dict service: %v", err)
	}
	resp, err := svc.ListDictTypes(context.Background(), &common_dict_v1.ListDictTypesRequest{CurrentPage: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list dict types: %v", err)
	}
	if resp.Page.Total != 1 || mustParseID(resp.DictTypes[0].Id) != selfType.Id {
		t.Fatalf("dict types = %+v, want only sender's", resp.DictTypes)
	}
}

func TestMethodPermissionsFailClosed(t *testing.T) {
	// 每个已登记方法都有非空码
	for method, codes := range methodPermissions {
		if len(codes) == 0 || codes[0] == "" {
			t.Fatalf("method %s has empty permission code", method)
		}
	}
	// 未登记方法返回空 → authz 拦截器 fail-closed
	if got := methodPerms("/sender.v1.MessageService/Unknown"); len(got) != 0 {
		t.Fatalf("unregistered method perms = %v, want empty", got)
	}
	// 关键方法的码以 sender 服务段开头且来自种子清单
	checks := map[string]string{
		"/sender.v1.MessageService/SendMessage":     "sender.record.send",
		"/sender.v1.TemplateService/CreateTemplate": "sender.template.create",
		"/sender.v1.MenuService/CreateMenu":         "sender.menu.create",
		"/sender.v1.DictService/ListDictItems":      "sender.dict.list",
	}
	for method, want := range checks {
		got := methodPerms(method)
		if len(got) != 1 || got[0] != want {
			t.Fatalf("method %s perms = %v, want [%s]", method, got, want)
		}
	}
	// ListMyMenuTree 是 auth-only，不判权
	if _, ok := authOnlyMethods["/sender.v1.MenuService/ListMyMenuTree"]; !ok {
		t.Fatal("ListMyMenuTree should be auth-only")
	}
}
