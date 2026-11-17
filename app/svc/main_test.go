package main

import "testing"

func TestCheckEnvValue(t *testing.T) {
	if err := checkEnvValue("JWT_SECRET", "a-real-secret"); err != nil {
		t.Fatalf("real value rejected: %v", err)
	}
	for name, v := range map[string]string{"empty": "", "ssm placeholder": ssmPlaceholder} {
		if err := checkEnvValue("JWT_SECRET", v); err == nil {
			t.Errorf("%s accepted, want error", name)
		}
	}
}
