package auth

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/config"
)

// AUTH_REGISTER_SKIP_EMAIL_VERIFY is a local/self-hosted opt-in: it must be
// off by default and never take effect in staging/production.
func TestRegisterEmailVerifyDisabled(t *testing.T) {
	cases := []struct {
		name   string
		cfg    *config.Config
		expect bool
	}{
		{"nil config", nil, false},
		{"default off", &config.Config{AppEnv: "development"}, false},
		{
			"opt-in development",
			&config.Config{AppEnv: "development", Auth: config.AuthConfig{RegisterSkipEmailVerify: true}},
			true,
		},
		{
			"opt-in ignored in production",
			&config.Config{AppEnv: "production", Auth: config.AuthConfig{RegisterSkipEmailVerify: true}},
			false,
		},
		{
			"opt-in ignored in staging",
			&config.Config{AppEnv: "staging", Auth: config.AuthConfig{RegisterSkipEmailVerify: true}},
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.RegisterEmailVerifyDisabled(); got != tc.expect {
				t.Fatalf("RegisterEmailVerifyDisabled() = %v, want %v", got, tc.expect)
			}
		})
	}
}
