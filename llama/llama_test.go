package llama

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

// https://github.com/ollama/ollama/issues/7978
const issue7978JSONSchema = `{
  "type": "object",
  "properties": {
    "steps": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "explanation": { "type": "string" },
          "output": { "type": "string" },
          "nested": {
            "type": "object",
            "properties": {
              "deep": { "type": "string" }
            }
          }
        },
        "required": ["explanation", "output"],
        "additionalProperties": false
      }
    },
    "final_answer": { "type": "string" },
    "01_numbered_key": { "type": "string" },
    "numbers": {
      "type": "array",
      "items": { "type": "number" }
    },
    "booleans": {
      "type": "array",
      "items": { "type": "boolean" }
    },
    "mixed": {
      "type": "array",
      "items": {
        "oneOf": [
          { "type": "string" },
          { "type": "number" },
          { "type": "boolean" }
        ]
      }
    }
  },
  "required": ["steps", "final_answer"],
  "additionalProperties": false
}`

func TestIssue7978(t *testing.T) {
	g := SchemaToGrammar([]byte(issue7978JSONSchema))
	if g == nil {
		t.Fatal("failed to convert JSON schema to grammar")
	}

	t.Logf("grammar:\n%s", g)
	t.Log()

	var got string
	s := bufio.NewScanner(bytes.NewReader(g))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		step, _, _ := strings.Cut(line, " ::= ")
		step = strings.TrimSpace(step)
		if step == "root" {
			got = line
		}
	}

	want := `root ::= "{" space steps-kv "," space final-answer-kv ( "," space ( 01-numbered-key-kv 01-numbered-key-rest | numbers-kv numbers-rest | booleans-kv booleans-rest | mixed-kv ) )? "}" space`
	if got != want {
		t.Errorf("root =\n%qwant:\n%q", got, want)
	}
}

func TestSchemaToGrammar(t *testing.T) {
	cases := []struct {
		schema string
		prefix []byte // nil is checked as nil
	}{
		{`invalid`, nil},

		// Simple heuristic/smoke test
		{`{"type":"object"}`, []byte("root ::= object")},
	}

	for _, c := range cases {
		t.Run(c.schema, func(t *testing.T) {
			g := SchemaToGrammar([]byte(c.schema))
			if c.prefix == nil && g != nil {
				t.Fatalf("grammar = %v, want nil", g)
			}
			if !bytes.HasPrefix(g, c.prefix) {
				t.Errorf("grammar = %q, want %q", g, c.prefix)
			}
		})
	}
}

// TestPatternWithMinMaxLength tests that when both pattern and minLength/maxLength
// are specified, the grammar correctly enforces both constraints.
// This is a regression test for the bug where minLength was ignored when pattern was present.
func TestPatternWithMinMaxLength(t *testing.T) {
	schema := `{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"pattern": "^[a-zA-Z]+$",
				"minLength": 10,
				"maxLength": 25
			}
		},
		"required": ["name"]
	}`

	g := SchemaToGrammar([]byte(schema))
	if g == nil {
		t.Fatal("failed to convert JSON schema to grammar")
	}

	grammarStr := string(g)
	t.Logf("Generated grammar:\n%s", grammarStr)

	// The grammar should contain {10,25} quantifier for the pattern
	// Before the fix, it would have just + (unbounded)
	if !strings.Contains(grammarStr, "{10,25}") {
		t.Errorf("grammar should contain {10,25} quantifier for minLength=10, maxLength=25")
		t.Errorf("This indicates minLength/maxLength are being ignored when pattern is present")
	}

	// Should NOT contain unbounded + for the letter pattern
	// (the pattern [a-zA-Z]+ should become [a-zA-Z]{10,25})
	lines := strings.Split(grammarStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "[a-zA-Z]+") && !strings.Contains(line, "{") {
			t.Errorf("found unbounded [a-zA-Z]+ pattern, should be bounded: %s", line)
		}
	}
}
