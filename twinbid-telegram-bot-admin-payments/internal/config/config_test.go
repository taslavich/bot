package config

import "testing"

func TestAllowedUserIDsContainsFailsClosedWhenEmpty(t *testing.T) {
	var ids AllowedUserIDs
	if ids.Contains(12345) {
		t.Fatal("empty allowlist must reject every Telegram user")
	}
}

func TestAllowedUserIDsSetValue(t *testing.T) {
	var ids AllowedUserIDs
	if err := ids.SetValue("12345, 67890"); err != nil {
		t.Fatalf("SetValue() error = %v", err)
	}
	if !ids.Contains(12345) || !ids.Contains(67890) {
		t.Fatalf("parsed allowlist = %#v", ids)
	}
	if ids.Contains(11111) {
		t.Fatal("unexpected Telegram user was allowed")
	}
}
