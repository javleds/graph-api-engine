package config

import "testing"

func TestOpenOriginsRequireExplicitDevelopmentMode(t *testing.T) {
	c, err := Parse(example(t))
	if err != nil {
		t.Fatal(err)
	}
	if c.Security.Egress.AllowAnyOriginForDevelopment {
		t.Fatal("origins open by default")
	}
	c.Security.Egress.AllowAnyOriginForDevelopment = true
	if c.Validate() == nil {
		t.Fatal("open origins allowed without development")
	}
	c.Security.Egress.AllowHTTPForDevelopment = true
	c.Security.Egress.UpstreamAllowedOrigins = nil
	c.Security.Egress.WebhookAllowedOrigins = nil
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if !c.DefinitionLimits().AllowAnyOriginForDevelopment {
		t.Fatal("missing compiler setting")
	}
	c.Security.Egress.AllowAnyOriginForDevelopment = false
	if c.Validate() == nil {
		t.Fatal("empty allowlists allowed without explicit opening")
	}
}
