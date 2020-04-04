package main

import (
	"log"
	"os"

	_ "github.com/mattn/go-oci8"

	plugin "github.com/hashicorp/vault-plugin-database-oracle"
	"github.com/hashicorp/vault/api"
)

func main() {
	apiClientMeta := &api.PluginAPIClientMeta{}
	flags := apiClientMeta.FlagSet()
	flags.Parse(os.Args[1:])

	err := plugin.Run(apiClientMeta.GetTLSConfig())
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
