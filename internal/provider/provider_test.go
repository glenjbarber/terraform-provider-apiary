package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories wires the real terraform binary on
// PATH (via TF_ACC=1, terraform-plugin-testing's resource.Test) to
// this provider's in-process implementation - no separate provider
// binary or registry install needed. Each acceptance test then points
// the provider at a local httptest server standing in for restshimd,
// the same fake-backend-not-fake-CLI approach Apiary's own reconciler/
// frontend tests use for their own fakes.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"apiary": providerserver.NewProtocol6WithError(New("test")()),
}
