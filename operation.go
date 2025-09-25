package blackstart

import (
	"log/slog"
	"slices"
)

// Operation represents a single operation in a Workflow. Each operation uses a specific module to
// configure resources. In an imperative Workflow, operations could be a step to perform in the
// Workflow. In Blackstart's declarative Workflow, the order of the operations is determined at
// runtime based on the dependencies between operations.
// --8<-- [start:Operation]
type Operation struct {
	// Module is the identifier value of the module that will be used to configure the resource.
	Module string

	// Id is a unique identifier for the operation. This is used to reference the operation in
	// other operations.
	Id string

	// Name is a human-readable name for the operation.
	Name string

	// Description is a human-readable description of the operation. Use this to provide more
	// context about the operation.
	Description string

	// DependsOn is a list of operation IDs that this operation depends on. The operations that
	// this operation depends on will be executed before this operation.
	DependsOn []string

	// Inputs are used to configure the module. The moduleContext are specific to each module and
	// are used to configure the module's behavior.
	Inputs map[string]Input

	// DoesNotExist is a special parameter that can be used to indicate that the resource should
	// not exist. This is useful for resources that are changed from a previous state and now
	// should be deleted if they still exist.
	DoesNotExist bool

	// Tainted is a special parameter that can be used to indicate that the resource is tainted and
	// should be replaced. This is useful for resources that always must be updated so that
	// attributes / output values are known by Blackstart. This should not be configured by users,
	// and should only be used explicitly by modules.
	Tainted bool
}

// --8<-- [end:Operation]

// setup performs basic pre-execution tasks for operations. It must be called on all operations
// before creating the directed graph of dependencies. The setup will walk through moduleContext and add
// any implicit dependencies to the DependsOn list of operations for the current operation.
func (o *Operation) setup() error {
	for _, v := range o.Inputs {
		if v.IsStatic() {
			continue
		}
		if !slices.Contains(o.DependsOn, v.DependencyId()) {
			o.DependsOn = append(o.DependsOn, v.DependencyId())
		}
	}
	return nil
}

func (o *Operation) execute(mctx ModuleContext, logger *slog.Logger) error {
	var m Module
	var err error
	var check bool

	logger.Debug("instantiating module for operation", "module", o.Module, "id", o.Id)
	m, err = NewModule(o)
	if err != nil {
		return err
	}

	logger.Info("operation check", "module", o.Module, "id", o.Id)
	check, err = m.Check(mctx)
	if err != nil {
		logger.Debug(
			"failed to check module",
			"module", o.Module,
			"id", o.Id,
			"inputs", o.Inputs,
			"error", err,
		)
	}
	if check {
		logger.Info("operation check passed", "module", o.Module, "id", o.Id)
		return nil
	}

	logger.Info("operation set", "module", o.Module, "id", o.Id)
	err = m.Set(mctx)
	if err != nil {
		logger.Warn("operation set failed", "module", o.Module, "id", o.Id, "error", err)
		return err
	}
	logger.Info("operation set passed", "module", o.Module, "id", o.Id)
	return nil
}
