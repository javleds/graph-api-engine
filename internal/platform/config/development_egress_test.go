package config

import "testing"

func TestDestinationPatternsRequireExplicitLists(t *testing.T) {
	c, err := Parse(example(t))
	if err != nil {
		t.Fatal(err)
	}
	c.Security.Egress.UpstreamAllowedOrigins = nil
	c.Security.Egress.WebhookAllowedOrigins = nil
	if c.Validate() == nil {
		t.Fatal("empty destination patterns accepted")
	}
	c.Security.Egress.UpstreamAllowedOrigins = []string{"*"}
	c.Security.Egress.WebhookAllowedOrigins = []string{"*"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}
