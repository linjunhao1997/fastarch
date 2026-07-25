package auth

import "testing"

func TestGenerateJWT(t *testing.T) {
	type args struct {
		param ClaimsParam
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateJWT(tt.args.param)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateJWT() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GenerateJWT() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAccountIdFromJwt(t *testing.T) {
	type args struct {
		tokenString string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetAccountIdFromJwt(tt.args.tokenString)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAccountIdFromJwt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetAccountIdFromJwt() got = %v, want %v", got, tt.want)
			}
		})
	}
}
