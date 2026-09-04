package alap

import (
	"github.com/joysriramsarkar/nilLang/pkg/alap/ai"
	"github.com/joysriramsarkar/nilLang/pkg/alap/core"
	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
	"github.com/joysriramsarkar/nilLang/pkg/alap/entity"
	"github.com/joysriramsarkar/nilLang/pkg/alap/onuron"
	"github.com/joysriramsarkar/nilLang/pkg/alap/routing"
	"github.com/joysriramsarkar/nilLang/pkg/alap/server"
	"github.com/joysriramsarkar/nilLang/pkg/alap/state"
	"github.com/joysriramsarkar/nilLang/pkg/alap/ui"
)

// Version of the Alap Framework
const Version = "0.2.0"

// NewApp creates a new Alap Application
func NewApp(name, version string) *core.App {
	return core.NewApp(name, version)
}

// NewStore creates a new reactive state store
func NewStore() *state.Store {
	return state.NewStore()
}

// NewRouter creates a new application router
func NewRouter() *routing.Router {
	return routing.NewRouter()
}

// NewPage creates a high-level UI Page
func NewPage(title string) *ui.Page {
	return ui.NewPage(title)
}

// NewEntity creates a new unified application model entity
func NewEntity(name string) *entity.Entity {
	return entity.NewEntity(name)
}

// NewServerService creates a new server service
func NewServerService(name, basePath string) *server.Service {
	return server.NewService(name, basePath)
}

// NewDataset creates a tabular dataset
func NewDataset(name string, columns []string) *data.Dataset {
	return data.NewDataset(name, columns)
}

// NewAIOracle creates an application truth oracle
func NewAIOracle() *ai.ApplicationTruth {
	return ai.NewApplicationTruth()
}

// NewOnuronAdapter creates an Onuron platform adapter
func NewOnuronAdapter() *onuron.Adapter {
	return onuron.NewAdapter()
}
