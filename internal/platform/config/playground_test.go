package config

import "testing"

func TestMetricsCannotShadowPlayground(t *testing.T) {
	for _, path := range []string{"/%70layground", "/%70layground/assets/test.js", "/playground", "/playground/", "/playground/api/executions", "/playground/assets/app.js", "/other/../playground"} {
		c, err := Parse(example(t))
		if err != nil {
			t.Fatal(err)
		}
		c.Observability.MetricsPath = path
		if err := c.Validate(); err == nil {
			t.Fatalf("accepted %s", path)
		}
	}
}
