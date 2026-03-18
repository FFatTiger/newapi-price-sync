package merger

import (
	"testing"

	"newapi-price-sync/internal/models"
)

func TestAliasesFor(t *testing.T) {
	aliases := aliasesFor("ZAI/GLM-4.7")
	set := map[string]bool{}
	for _, a := range aliases {
		set[a] = true
	}
	for _, want := range []string{"zai/glm-4.7", "glm-4.7", "zaiglm47", "glm47"} {
		if !set[want] {
			t.Fatalf("missing alias %q in %#v", want, aliases)
		}
	}
}

func TestMergeMatchesPrefixAndSeparators(t *testing.T) {
	current := models.NewPriceFields()
	current.ModelRatio["glm4.7"] = 1
	current.ModelRatio["GLM-4.7"] = 2

	incoming := models.NewPriceFields()
	incoming.ModelRatio["zai/glm4.7"] = 9.9

	merged := Merge(current, []models.PriceFields{incoming}, nil, nil, true)

	if got := merged.ModelRatio["glm4.7"]; got != 9.9 {
		t.Fatalf("expected glm4.7 updated to 9.9, got %v", got)
	}
	if got := merged.ModelRatio["GLM-4.7"]; got != 9.9 {
		t.Fatalf("expected GLM-4.7 updated to 9.9, got %v", got)
	}
	if _, exists := merged.ModelRatio["zai/glm4.7"]; exists {
		t.Fatalf("expected no extra alias key to be created when existing variants already match")
	}
}

func TestMergeCreatesOriginalKeyWhenNoMatch(t *testing.T) {
	merged := Merge(models.NewPriceFields(), []models.PriceFields{{
		ModelRatio: map[string]float64{"zai/glm4.7": 3.3},
	}}, nil, nil, true)

	if got := merged.ModelRatio["zai/glm4.7"]; got != 3.3 {
		t.Fatalf("expected original key created, got %v", got)
	}
}
