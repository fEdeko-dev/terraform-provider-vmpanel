// Провайдер Terraform для панели управления виртуальными машинами: машины, вебхуки, справочники.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/fEdeko-dev/terraform-provider-vmpanel/internal/provider"
)

var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "запуск под отладчиком")
	flag.Parse()
	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/fedeko-dev/vmpanel",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
