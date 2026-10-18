// Package firebaseauth 以 Firebase Admin SDK 實作 domain/user.TokenVerifier。
// ID token 驗證只需 project ID（簽章對 Google 公開金鑰驗證），不需 service account。
package firebaseauth

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"

	"github.com/as130232/busy-bee/busy-bee-be/domain/user"
)

type Verifier struct {
	client *auth.Client
}

var _ user.TokenVerifier = (*Verifier)(nil)

func New(ctx context.Context, projectID string) (*Verifier, error) {
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		return nil, fmt.Errorf("firebaseauth.New app: %w", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebaseauth.New auth client: %w", err)
	}
	return &Verifier{client: client}, nil
}

func (v *Verifier) Verify(ctx context.Context, idToken string) (user.Identity, error) {
	token, err := v.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return user.Identity{}, fmt.Errorf("firebaseauth.Verify: %w", err)
	}
	return user.Identity{
		UID:     token.UID,
		Email:   claimString(token.Claims, "email"),
		Name:    claimString(token.Claims, "name"),
		Picture: claimString(token.Claims, "picture"),
		// Google 登入的 token 一律帶 email_verified=true；缺少或非 bool 視為未驗證（fail-closed）。
		EmailVerified: claimBool(token.Claims, "email_verified"),
	}, nil
}

func claimString(claims map[string]any, key string) string {
	v, _ := claims[key].(string)
	return v
}

func claimBool(claims map[string]any, key string) bool {
	v, _ := claims[key].(bool)
	return v
}
