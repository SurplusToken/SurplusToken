package service

import "testing"

func TestCreateOpenAITestPayloadStoreSwitch(t *testing.T) {
	oauth := createOpenAITestPayload("gpt-5.6-sol", true)
	if v, ok := oauth["store"]; !ok || v != false {
		t.Fatalf("OAuth payload must carry store:false, got %v", oauth["store"])
	}

	apiKey := createOpenAITestPayload("gpt-5.6-sol", false)
	if _, ok := apiKey["store"]; ok {
		t.Fatalf("API-key payload without the flag must omit store")
	}

	if !accountForcesTestStoreFalse(&Account{Extra: map[string]any{"openai_test_store_false": true}}) {
		t.Fatalf("bool true flag must force store:false")
	}
	if !accountForcesTestStoreFalse(&Account{Extra: map[string]any{"openai_test_store_false": "true"}}) {
		t.Fatalf("string true flag must force store:false")
	}
	if accountForcesTestStoreFalse(&Account{Extra: map[string]any{"openai_test_store_false": false}}) {
		t.Fatalf("explicit false flag must not force store:false")
	}
	if accountForcesTestStoreFalse(nil) || accountForcesTestStoreFalse(&Account{}) {
		t.Fatalf("nil account or missing flag must not force store:false")
	}
}
