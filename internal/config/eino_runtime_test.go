package config

import "testing"

func TestEinoDefinitionDefaultOff(t *testing.T) {
	c := &Config{}
	if _, ok := c.EinoDefinition("v2"); ok {
		t.Fatal("default must remain legacy")
	}
}

func TestEinoThirdPublishedDefinition(t *testing.T) {
	c := &Config{}
	if err := c.loadEinoDefinitions(`{"summary":"knowledge-summary-v1"}`); err != nil {
		t.Fatalf("third published definition rejected: %v", err)
	}
	if d, ok := c.EinoDefinition("summary"); !ok || d != "knowledge-summary-v1" {
		t.Fatal("third definition binding missing")
	}
}
func TestEinoDefinitionSelection(t *testing.T) {
	c := &Config{}
	if err := c.loadEinoDefinitions(`{"v2":"contract-review-v1"}`); err != nil {
		t.Fatal(err)
	}
	if d, ok := c.EinoDefinition("v2"); !ok || d != "contract-review-v1" {
		t.Fatal("missing exact binding")
	}
	for _, id := range []string{"", "other", "854499ab-7e55-4be0-a681-807a096fea4e"} {
		if _, ok := c.EinoDefinition(id); ok {
			t.Fatalf("unexpected binding %q", id)
		}
	}
}

func TestEinoCreditDefinitionSelection(t *testing.T) {
	c := &Config{}
	if err := c.loadEinoDefinitions(`{"credit":"customer-credit-readonly-v1","v2":"contract-review-v1"}`); err != nil {
		t.Fatal(err)
	}
	if d, ok := c.EinoDefinition("credit"); !ok || d != "customer-credit-readonly-v1" {
		t.Fatal("credit definition missing")
	}
}
func TestEinoDefinitionInvalid(t *testing.T) {
	for _, s := range []string{`null`, `[]`, `{"":"contract-review-v1"}`, `{"v2":"unknown"}`, `{"v2":""}`, `{"854499ab-7e55-4be0-a681-807a096fea4e":"contract-review-v1"}`, `{"v2":"contract-review-v1","v2":"contract-review-v1"}`, `{`} {
		t.Run(s, func(t *testing.T) {
			c := &Config{}
			if err := c.loadEinoDefinitions(s); err == nil {
				t.Fatal("invalid mapping accepted")
			}
		})
	}
}
