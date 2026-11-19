package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"
)

func TestAdmittedAppIdsJWTRoundTrip(t *testing.T) {
	admitted := []int64{2, 7, 9}
	token, err := GenerateJWT(ClaimsParam{
		UserId:         "100",
		Username:       "hao",
		ExpirationTime: time.Now().Add(time.Hour),
		AdmittedAppIds: admitted,
	})
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}
	claims, err := ParseClaims(token)
	if err != nil {
		t.Fatalf("ParseClaims: %v", err)
	}
	if len(claims.AdmittedAppIds) != len(admitted) {
		t.Fatalf("admitted len = %d, want %d", len(claims.AdmittedAppIds), len(admitted))
	}
	for i := range admitted {
		if claims.AdmittedAppIds[i] != admitted[i] {
			t.Fatalf("admitted[%d] = %d, want %d", i, claims.AdmittedAppIds[i], admitted[i])
		}
	}
}

func TestIsAppAdmitted(t *testing.T) {
	user := &CustomClaims{AdmittedAppIds: []int64{2, 7}}
	admin := &CustomClaims{IsAdmin: true}

	cases := []struct {
		name   string
		claims *CustomClaims
		appId  int64
		want   bool
	}{
		{"admitted hit", user, 7, true},
		{"admitted miss", user, 9, false},
		{"zero app id", user, 0, false},
		{"nil claims", nil, 2, false},
		{"admin any app", admin, 999, true},
		{"admin zero app", admin, 0, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAppAdmitted(tt.claims, tt.appId); got != tt.want {
				t.Fatalf("IsAppAdmitted = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAppAdmittedDefensiveCopy(t *testing.T) {
	admitted := []int64{2, 7}
	token, err := GenerateJWT(ClaimsParam{ExpirationTime: time.Now().Add(time.Hour), AdmittedAppIds: admitted})
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}
	admitted[0] = 999 // 生成后修改入参不得影响已签发令牌
	claims, err := ParseClaims(token)
	if err != nil {
		t.Fatalf("ParseClaims: %v", err)
	}
	if claims.AdmittedAppIds[0] != 2 {
		t.Fatalf("claim mutated by caller: got %d, want 2", claims.AdmittedAppIds[0])
	}
}

func TestAppCodeFromMetadata(t *testing.T) {
	if got := AppCodeFromMetadata(metadata.Pairs(MetadataAppCode, "order")); got != "order" {
		t.Fatalf("app code = %q, want order", got)
	}
	if got := AppCodeFromMetadata(metadata.MD{}); got != "" {
		t.Fatalf("missing app code = %q, want empty", got)
	}
}

func TestResolveAppCode(t *testing.T) {
	// 解析器：code→id
	resolver := func(_ context.Context, code string) (int64, error) {
		switch code {
		case "order":
			return 2, nil
		case "approval":
			return 3, nil
		default:
			return 0, errors.New("unknown app code")
		}
	}
	user := &CustomClaims{AdmittedAppIds: []int64{2}}

	cases := []struct {
		name   string
		code   string
		claims *CustomClaims
		wantId int64
		wantErr bool
	}{
		{"admitted", "order", user, 2, false},
		{"not admitted", "approval", user, 0, true},
		{"unknown code", "nope", user, 0, true},
		{"admin any app", "approval", &CustomClaims{IsAdmin: true}, 3, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveAppCode(tt.code, tt.claims, resolver)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveAppCode error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.wantId {
				t.Fatalf("appId = %d, want %d", got, tt.wantId)
			}
		})
	}
}

func TestResolveAppCodeNoResolverFailClosed(t *testing.T) {
	if _, err := resolveAppCode("order", &CustomClaims{AdmittedAppIds: []int64{2}}, nil); err != ErrAppNotAdmitted {
		t.Fatalf("nil resolver error = %v, want ErrAppNotAdmitted", err)
	}
}

func TestDataAppIdContext(t *testing.T) {
	ctx := context.Background()
	if got := GetDataAppId(ctx); got != 0 {
		t.Fatalf("unset data app id = %d, want 0", got)
	}
	ctx = WithDataAppId(ctx, 7)
	if got := GetDataAppId(ctx); got != 7 {
		t.Fatalf("data app id = %d, want 7", got)
	}
	// 与宿主固定 appId 正交：各取各的 key，互不覆盖。
	ctx = WithApplicationId(ctx, 1)
	if GetApplicationId(ctx) != 1 || GetDataAppId(ctx) != 7 {
		t.Fatalf("context keys collided: host=%d data=%d", GetApplicationId(ctx), GetDataAppId(ctx))
	}
}
