package main

import (
	"testing"
	"time"
)

func TestParseRateLimitJSON_ValidResponse(t *testing.T) {
	data := []byte(`{"resources":{"core":{"remaining":4999,"limit":5000,"reset":1700000000},"graphql":{"remaining":4900,"limit":5000,"reset":1700001000}}}`)
	rl := parseRateLimitJSON(data)
	if rl == nil {
		t.Fatal("expected non-nil RateLimit")
	}
	if rl.Remaining != 4999 {
		t.Errorf("Remaining = %d, want 4999", rl.Remaining)
	}
	if rl.Limit != 5000 {
		t.Errorf("Limit = %d, want 5000", rl.Limit)
	}
	want := time.Unix(1700000000, 0).UTC()
	if !rl.ResetAt.Equal(want) {
		t.Errorf("ResetAt = %v, want %v", rl.ResetAt, want)
	}
	if rl.GraphQLRemaining != 4900 {
		t.Errorf("GraphQLRemaining = %d, want 4900", rl.GraphQLRemaining)
	}
	if rl.GraphQLLimit != 5000 {
		t.Errorf("GraphQLLimit = %d, want 5000", rl.GraphQLLimit)
	}
	gqlWant := time.Unix(1700001000, 0).UTC()
	if !rl.GraphQLResetAt.Equal(gqlWant) {
		t.Errorf("GraphQLResetAt = %v, want %v", rl.GraphQLResetAt, gqlWant)
	}
}

func TestParseRateLimitJSON_ZeroRemaining(t *testing.T) {
	data := []byte(`{"resources":{"core":{"remaining":0,"limit":5000,"reset":1700000000}}}`)
	rl := parseRateLimitJSON(data)
	if rl == nil {
		t.Fatal("expected non-nil RateLimit for zero remaining")
	}
	if rl.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0", rl.Remaining)
	}
	if rl.Limit != 5000 {
		t.Errorf("Limit = %d, want 5000", rl.Limit)
	}
}

func TestParseRateLimitJSON_EmptyCoreFields(t *testing.T) {
	// When core exists but fields are missing, Go zero-values apply.
	// This documents the edge case: Limit=0 and Remaining=0 with epoch time.
	data := []byte(`{"resources":{"core":{}}}`)
	rl := parseRateLimitJSON(data)
	if rl == nil {
		t.Fatal("expected non-nil RateLimit (zero-value struct)")
	}
	if rl.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0", rl.Remaining)
	}
	if rl.Limit != 0 {
		t.Errorf("Limit = %d, want 0", rl.Limit)
	}
	epoch := time.Unix(0, 0).UTC()
	if !rl.ResetAt.Equal(epoch) {
		t.Errorf("ResetAt = %v, want epoch %v", rl.ResetAt, epoch)
	}
}

func TestParseRateLimitJSON_InvalidJSON(t *testing.T) {
	data := []byte(`not json at all`)
	rl := parseRateLimitJSON(data)
	if rl != nil {
		t.Errorf("expected nil for invalid JSON, got %+v", rl)
	}
}

func TestParseRateLimitJSON_EmptyInput(t *testing.T) {
	rl := parseRateLimitJSON([]byte(""))
	if rl != nil {
		t.Errorf("expected nil for empty input, got %+v", rl)
	}
}

func TestParseRateLimitJSON_NilInput(t *testing.T) {
	rl := parseRateLimitJSON(nil)
	if rl != nil {
		t.Errorf("expected nil for nil input, got %+v", rl)
	}
}

func TestParseRateLimitJSON_MissingResourcesKey(t *testing.T) {
	// Valid JSON but wrong structure — Go unmarshals to zero values.
	// Documents that this returns a non-nil RateLimit with all zeros.
	data := []byte(`{"rate":{"limit":5000}}`)
	rl := parseRateLimitJSON(data)
	if rl == nil {
		t.Fatal("expected non-nil RateLimit (zero-value struct from mismatched keys)")
	}
	if rl.Remaining != 0 || rl.Limit != 0 {
		t.Errorf("expected zero values, got Remaining=%d, Limit=%d", rl.Remaining, rl.Limit)
	}
}

func TestParseRateLimitJSON_LargeValues(t *testing.T) {
	data := []byte(`{"resources":{"core":{"remaining":14999,"limit":15000,"reset":1800000000}}}`)
	rl := parseRateLimitJSON(data)
	if rl == nil {
		t.Fatal("expected non-nil RateLimit")
	}
	if rl.Remaining != 14999 {
		t.Errorf("Remaining = %d, want 14999", rl.Remaining)
	}
	if rl.Limit != 15000 {
		t.Errorf("Limit = %d, want 15000", rl.Limit)
	}
}

func TestParseRateLimitJSON_CoreOnlyNoGraphQL(t *testing.T) {
	// When graphql key is absent, GraphQL fields should be zero-valued.
	data := []byte(`{"resources":{"core":{"remaining":4999,"limit":5000,"reset":1700000000}}}`)
	rl := parseRateLimitJSON(data)
	if rl == nil {
		t.Fatal("expected non-nil RateLimit")
	}
	if rl.Remaining != 4999 {
		t.Errorf("Remaining = %d, want 4999", rl.Remaining)
	}
	if rl.GraphQLRemaining != 0 {
		t.Errorf("GraphQLRemaining = %d, want 0", rl.GraphQLRemaining)
	}
	if rl.GraphQLLimit != 0 {
		t.Errorf("GraphQLLimit = %d, want 0", rl.GraphQLLimit)
	}
}
