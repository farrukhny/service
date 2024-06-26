// Package reporting binds the reporting domain set of routes into the specified app.
package reporting

import (
	"github.com/ardanlabs/service/apis/services/salesapiweb/routes/sys/checkapi"
	"github.com/ardanlabs/service/apis/services/salesapiweb/routes/views/vproductapi"
	"github.com/ardanlabs/service/app/api/mux"
	"github.com/ardanlabs/service/foundation/web"
)

// Routes constructs the add value which provides the implementation of
// of RouteAdder for specifying what routes to bind to this instance.
func Routes() add {
	return add{}
}

type add struct{}

// Add implements the RouterAdder interface.
func (add) Add(app *web.App, cfg mux.Config) {
	checkapi.Routes(app, checkapi.Config{
		Build: cfg.Build,
		Log:   cfg.Log,
		DB:    cfg.DB,
	})

	vproductapi.Routes(app, vproductapi.Config{
		VProductBus: cfg.BusView.Product,
		Auth:        cfg.Auth,
	})
}
