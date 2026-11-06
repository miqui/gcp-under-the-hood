package main

import "testing"

func TestClassify(t *testing.T) {
	var s Stats
	for _, c := range []int{200, 204, 302, 500, 503, 404, 0} {
		s.Classify(c)
	}
	if s.OK.Load() != 3 || s.ServerErr.Load() != 2 || s.Other.Load() != 1 || s.NetErr.Load() != 1 {
		t.Fatalf("%s", s.String())
	}
	want := "total=7 ok=3 5xx=2 4xx=1 neterr=1 bad%=42.86"
	if s.String() != want {
		t.Fatalf("got %q want %q", s.String(), want)
	}
}
