package logic

import (
	"testing"
	"time"
)

func TestIAMDBUser(t *testing.T) {
	got, err := IAMDBUser("dt06-app@proj.iam.gserviceaccount.com")
	if err != nil || got != "dt06-app@proj.iam" {
		t.Fatalf("got %q, %v", got, err)
	}
	for _, bad := range []string{"", "user@example.com", "nope.gserviceaccount.com"} {
		if _, err := IAMDBUser(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestValidateExpiry(t *testing.T) {
	cases := map[time.Duration]bool{
		0: false, -time.Second: false, time.Minute: true,
		MaxV4Expiry: true, MaxV4Expiry + time.Second: false,
	}
	for d, ok := range cases {
		if (ValidateExpiry(d) == nil) != ok {
			t.Errorf("duration %s: ok=%v", d, ok)
		}
	}
}

func TestEnv(t *testing.T) {
	get := func(k string) string { return map[string]string{"A": "x"}[k] }
	if v, err := Env(get, "A"); err != nil || v != "x" {
		t.Fatal(v, err)
	}
	if _, err := Env(get, "B"); err == nil {
		t.Fatal("want error")
	}
}
