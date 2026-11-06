package supply

import (
	"strings"
	"testing"
)

var good = "us-docker.pkg.dev/p/r/app@sha256:" + strings.Repeat("a", 64)

func TestParseDigestRef(t *testing.T) {
	r, err := ParseDigestRef(good)
	if err != nil {
		t.Fatal(err)
	}
	if r.Registry != "us-docker.pkg.dev" || r.Path != "p/r/app" || r.String() != good {
		t.Fatalf("bad parse: %+v", r)
	}
	bad := []string{
		"us-docker.pkg.dev/p/r/app:latest",
		"us-docker.pkg.dev/p/r/app@sha256:abc",
		"us-docker.pkg.dev/p/r/app@sha256:" + strings.Repeat("A", 64),
		"app@sha256:" + strings.Repeat("a", 64),
		"/x@sha256:" + strings.Repeat("a", 64),
	}
	for _, b := range bad {
		if _, err := ParseDigestRef(b); err == nil {
			t.Errorf("expected error for %q", b)
		}
	}
}

func TestPayloadRoundTrip(t *testing.T) {
	r, _ := ParseDigestRef(good)
	data, err := BuildPayload(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"docker-manifest-digest":"sha256:`) {
		t.Fatalf("unexpected json %s", data)
	}
	if err := ValidatePayload(data, r); err != nil {
		t.Fatal(err)
	}
	other, _ := ParseDigestRef("us-docker.pkg.dev/p/r/app@sha256:" + strings.Repeat("b", 64))
	if err := ValidatePayload(data, other); err == nil {
		t.Fatal("expected digest mismatch")
	}
	if err := ValidatePayload([]byte("{"), r); err == nil {
		t.Fatal("expected json error")
	}
	if err := ValidatePayload([]byte(`{"critical":{"type":"x"}}`), r); err == nil {
		t.Fatal("expected type error")
	}
}

func TestValidateSplits(t *testing.T) {
	if err := ValidateSplits([]Split{{"a", 75}, {"b", 25}}); err != nil {
		t.Fatal(err)
	}
	if ValidateSplits([]Split{{"a", 70}, {"b", 25}}) == nil {
		t.Fatal("sum != 100 should fail")
	}
	if ValidateSplits([]Split{{"a", 101}, {"b", -1}}) == nil {
		t.Fatal("range should fail")
	}
}
