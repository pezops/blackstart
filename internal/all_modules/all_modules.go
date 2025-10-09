package all_modules

// This package is used to import all modules so that they are registered
import (
	_ "github.com/pezops/blackstart/modules/google/cloud"
	_ "github.com/pezops/blackstart/modules/google/cloudsql"
	_ "github.com/pezops/blackstart/modules/kubernetes"
	_ "github.com/pezops/blackstart/modules/mock"
	_ "github.com/pezops/blackstart/modules/postgres"
)
