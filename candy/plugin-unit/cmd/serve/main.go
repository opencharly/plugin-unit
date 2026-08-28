// Command serve is the OUT-OF-PROCESS entrypoint for the unit kit check verb: a thin
// shim serving the importable verb over go-plugin gRPC via sdk.ServeCheckVerb, which
// reconstructs the kit.CheckContext from the host's reverse channel. The SAME verb
// compiles INTO charly in-process when listed in compiled_plugins; this binary is
// host-built + connected only when it is not — placement is invisible above the registry.
package main

import (
	unit "github.com/opencharly/plugin-unit/candy/plugin-unit"
	"github.com/opencharly/sdk"
)

func main() { sdk.ServeCheckVerb(unit.NewCheckVerb(), unit.NewMeta()) }
