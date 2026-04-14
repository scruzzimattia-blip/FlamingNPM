package waf

import (
	"regexp"
	"testing"

	"github.com/flamingnpm/waf/internal/models"
)

func TestAccumulateBlockScore(t *testing.T) {
	rules := []compiledRule{
		{
			rule: models.FirewallRule{
				Name:        "R1",
				Target:      "param",
				ScoreWeight: 20,
			},
			regex: regexp.MustCompile(`bad`),
		},
		{
			rule: models.FirewallRule{
				Name:        "R2",
				Target:      "param",
				ScoreWeight: 15,
			},
			regex: regexp.MustCompile(`evil`),
		},
	}

	score, names, _ := accumulateBlockScore(rules, "/x", "bad=1&evil=1", "", "")
	if score != 35 {
		t.Fatalf("score: erwartet 35, ist %d", score)
	}
	if len(names) != 2 {
		t.Fatalf("namen: erwartet 2, ist %d", len(names))
	}

	score2, _, _ := accumulateBlockScore(rules, "/x", "ok=1", "", "")
	if score2 != 0 {
		t.Fatalf("score ohne Treffer: erwartet 0, ist %d", score2)
	}
}

func TestScoreWeightDefault(t *testing.T) {
	rules := []compiledRule{
		{
			rule: models.FirewallRule{
				Name:        "R",
				Target:      "param",
				ScoreWeight: 0,
			},
			regex: regexp.MustCompile(`x`),
		},
	}
	score, _, _ := accumulateBlockScore(rules, "/", "x=1", "", "")
	if score != 10 {
		t.Fatalf("default-gewicht: erwartet 10, ist %d", score)
	}
}
