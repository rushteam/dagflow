package varfunc

import (
	"encoding/json"
	"testing"
	"time"
)

func TestConvertSyntax(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple var", `${DT}`, `{{.DT}}`},
		{"two vars", `${A}-${B}`, `{{.A}}-{{.B}}`},
		{"func no args", `${uuid()}`, `{{uuid}}`},
		{"func one arg", `${timestamp(0d)}`, `{{timestamp "0d"}}`},
		{"func two args", `${dateFormat(yyyyMMdd, -1d)}`, `{{dateFormat "yyyyMMdd" "-1d"}}`},
		{"native go template", `{{.X}}`, `{{.X}}`},
		{"mixed", `${DT}-{{.X}}`, `{{.DT}}-{{.X}}`},
		{"no placeholder", `plain text`, `plain text`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertSyntax(tt.in)
			if got != tt.want {
				t.Errorf("ConvertSyntax(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRenderPayload(t *testing.T) {
	now := time.Date(2025, 3, 15, 10, 30, 0, 0, time.UTC)

	varDefs := json.RawMessage(`[{"key":"DT","default_value":"2000-01-01"},{"key":"LIMIT","default_value":"100"}]`)
	vars := map[string]string{"DT": "2025-03-15"}

	t.Run("simple var substitution", func(t *testing.T) {
		payload := json.RawMessage(`{"date":"${DT}","limit":"${LIMIT}"}`)
		got := string(RenderPayload(payload, varDefs, vars, now))
		want := `{"date":"2025-03-15","limit":"100"}`
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})

	t.Run("inline dateFormat function", func(t *testing.T) {
		payload := json.RawMessage(`{"date":"${dateFormat(yyyyMMdd, -1d)}"}`)
		got := string(RenderPayload(payload, nil, nil, now))
		if got != `{"date":"20250314"}` {
			t.Errorf("got %s", got)
		}
	})

	t.Run("inline uuid function", func(t *testing.T) {
		payload := json.RawMessage(`{"id":"${uuid()}"}`)
		got := string(RenderPayload(payload, nil, nil, now))
		if len(got) < 40 {
			t.Errorf("expected UUID in result, got %s", got)
		}
	})

	t.Run("inline timestamp function", func(t *testing.T) {
		payload := json.RawMessage(`{"ts":"${timestamp(0d)}"}`)
		got := string(RenderPayload(payload, nil, nil, now))
		want := `{"ts":"1742034600"}`
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})

	t.Run("mixed vars and functions", func(t *testing.T) {
		payload := json.RawMessage(`{"date":"${DT}","yesterday":"${dateFormat(yyyy-MM-dd, -1d)}"}`)
		got := string(RenderPayload(payload, varDefs, vars, now))
		want := `{"date":"2025-03-15","yesterday":"2025-03-14"}`
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})

	t.Run("no placeholders passthrough", func(t *testing.T) {
		payload := json.RawMessage(`{"static":"value"}`)
		got := string(RenderPayload(payload, varDefs, vars, now))
		if got != `{"static":"value"}` {
			t.Errorf("got %s", got)
		}
	})

	t.Run("native go template syntax", func(t *testing.T) {
		payload := json.RawMessage(`{"date":"{{.DT}}"}`)
		got := string(RenderPayload(payload, varDefs, vars, now))
		want := `{"date":"2025-03-15"}`
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})
}

func TestResolveOverrides(t *testing.T) {
	now := time.Date(2025, 3, 15, 10, 30, 0, 0, time.UTC)

	t.Run("fixed values", func(t *testing.T) {
		raw := json.RawMessage(`[{"key":"DT","value":"2025-01-01"},{"key":"N","value":"500"}]`)
		got := ResolveOverrides(raw, now)
		if got["DT"] != "2025-01-01" || got["N"] != "500" {
			t.Errorf("got %v", got)
		}
	})

	t.Run("expression values", func(t *testing.T) {
		raw := json.RawMessage(`[{"key":"DT","value":"${dateFormat(yyyyMMdd, -1d)}"},{"key":"ID","value":"${uuid()}"}]`)
		got := ResolveOverrides(raw, now)
		if got["DT"] != "20250314" {
			t.Errorf("DT = %q, want 20250314", got["DT"])
		}
		if len(got["ID"]) != 36 {
			t.Errorf("ID should be UUID, got %q", got["ID"])
		}
	})

	t.Run("nil input", func(t *testing.T) {
		got := ResolveOverrides(nil, now)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("empty array", func(t *testing.T) {
		got := ResolveOverrides(json.RawMessage(`[]`), now)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("mixed fixed and expressions", func(t *testing.T) {
		raw := json.RawMessage(`[{"key":"DT","value":"${dateFormat(yyyy-MM-dd, 0d)}"},{"key":"LIMIT","value":"100"}]`)
		got := ResolveOverrides(raw, now)
		if got["DT"] != "2025-03-15" {
			t.Errorf("DT = %q, want 2025-03-15", got["DT"])
		}
		if got["LIMIT"] != "100" {
			t.Errorf("LIMIT = %q, want 100", got["LIMIT"])
		}
	})
}
