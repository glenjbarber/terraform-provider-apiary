// Command terraform-provider-apiary is the Terraform provider binary
// for Apiary. Run via Terraform itself (or `go run .` with a dev
// override in ~/.terraformrc during development) - not intended to be
// invoked directly by a human.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/glenjbarber/terraform-provider-apiary/internal/provider"
)

// version is set via -ldflags at build/release time (e.g.
// -X main.version=1.2.3 by goreleaser); "dev" is the local-build
// default.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/glenjbarber/apiary",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
