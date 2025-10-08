package blackstart

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"
)

var registeredModuleFactories = make(map[string]func() Module)
var registeredPathNames = make(map[string]string)
var ErrInputDoesNotExist = errors.New("input does not exist")
var _ ModuleContext = &moduleContext{}

type ModuleContextFlag int

const (
	TaintedFlag ModuleContextFlag = iota
	DoesNotExistFlag
)

type inputType int

// OutputValue is a structure that describes a value used in a module's outputs.
type OutputValue struct {
	// Description is a short description of the value. This is used to provide context
	// about what the value is and how it should be used.
	Description string

	// Type is the type of the value. This is used to provide context about what type
	// of value is expected.
	Type reflect.Type
}

// InputValue is a structure that describes a value used in a module's inputs.
type InputValue struct {
	// Description is a short description of the value. This is used to provide context about what
	// the value is and how it should be used.
	Description string

	// Type is the type of the value. This is used to provide context about what type of value is
	// expected.
	Type reflect.Type

	// Required indicates whether the value is required. When used as an input, this indicates
	// that the value must be provided for the module to function correctly.
	Required bool

	// Default is an optional default value for the input parameter.
	Default any
}

// ModuleInfo is a static structure that provides information about a module. It is used to
// provide metadata about the module, such as its name, description, and the inputs it requires
// and outputs it provides.
type ModuleInfo struct {
	// Id is the identifier of the module. This is used to identify the module in the
	// configuration and in the logs.
	Id string

	// Name is the name of the module. This is a human-readable identifier.
	Name string

	// Description is a short description of the module. This is used to provide context about
	// what the module does and how it should be used.
	Description string

	// Inputs is a map of input names to their configuration. This is used to provide context
	// about what inputs the module requires and how they should be used.
	Inputs map[string]InputValue

	// Outputs is a map of output names to their descriptions. This is used to provide context
	// about what outputs the module provides and how they should be used.
	Outputs map[string]OutputValue

	// Examples is a map of example titles to their YAML implementations. This is used to provide
	// users with a quick way to understand how to use the module.
	Examples map[string]string
}

// Module is the interface that all modules must implement. Modules are used to configure resources
// in various systems, but they all provide the same "check then set" interface. When running a
// job, blackstart will orchestrate the execution of operations and the modules they use.
// --8<-- [start:Module]
type Module interface {
	// Info returns a ModuleInfo structure that provides information about the module. This is used
	// to provide context about the module and its capabilities.
	Info() ModuleInfo

	// Validate is used by the module to validate that the Operation settings and parameters are
	// valid. If the Operation is invalid, then an error should be returned.
	Validate(op Operation) error

	// Check should be a safe, non-destructive method to ensure the expected state exists. If it
	// does not exist, Check must return false. If an alternate error is encountered while checking
	// the state, then an error should also be returned.
	Check(ctx ModuleContext) (bool, error)

	// Set configures the expected state if Check returns false or if tainted.
	Set(ctx ModuleContext) error
}

// --8<-- [end:Module]

// --8<-- [start:ModuleContext]
type ModuleContext interface {
	context.Context
	Input(key string) (Input, error)
	Output(key string, value interface{}) error
	DoesNotExist() bool
	Tainted() bool
}

// --8<-- [end:ModuleContext]

// newModuleContext creates a new module context from the provided context and operation.
func newModuleContext(ctx context.Context, op *Operation) *moduleContext {
	iv := make(map[string]Input)
	for k, v := range op.Inputs {
		if v.IsStatic() {
			iv[k] = v
		}
	}

	module, ok := registeredModuleFactories[op.Module]
	if !ok {
		panic(fmt.Errorf("module %s is not registered", op.Module))
	}
	info := module().Info()
	for name, param := range info.Inputs {
		if _, ok := iv[name]; !ok {
			// If the default is not nil or the input is not required (then it may be nil), set the default value.
			if param.Default != nil || !param.Required {
				iv[name] = NewInputFromValue(param.Default)
			}
		}
	}

	return &moduleContext{
		ctx:          ctx,
		inputValues:  iv,
		outputValues: make(map[string]interface{}),
		dne:          op.DoesNotExist,
		tainted:      op.Tainted,
	}
}

// moduleContext is the context passed to modules when they are executed. It provides access to
// the inputs and outputs of the module, as well as the context.Context methods.
type moduleContext struct {
	ctx          context.Context
	inputValues  map[string]Input
	outputValues map[string]interface{}
	dne          bool
	tainted      bool
}

// setInput is used to set an input value in the module context. This is primarily used to set a
// value from a dependency.
func (mc *moduleContext) setInput(key string, value interface{}) {
	mc.inputValues[key] = NewInputFromValue(value)
}

//// setOutput is used to set an output value in the module context after the module has been executed.
//func (mc *moduleContext) setOutput(key string, value interface{}) {
//	mc.outputValues[key] = value
//}

// getOutput is used to get an output value from the module context. This is primarily used to get
// a value from a dependency's context.
func (mc *moduleContext) getOutput(key string) (interface{}, error) {
	if _, ok := mc.outputValues[key]; !ok {
		return nil, fmt.Errorf("output key does not exist: %v", key)
	}
	return mc.outputValues[key], nil
}

// Output is used by modules to set output values. If the key already exists, an error is returned.
func (mc *moduleContext) Output(key string, value interface{}) error {
	if _, ok := mc.outputValues[key]; ok {
		return fmt.Errorf("output key already exists: %v", key)
	}
	mc.outputValues[key] = value
	return nil
}

// Input is used by modules to get input values. If the key does not exist, an error is returned.
func (mc *moduleContext) Input(key string) (Input, error) {
	if v, ok := mc.inputValues[key]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("input key: %v: %w", key, ErrInputDoesNotExist)
}

// DoesNotExist returns true if the operation is being run in "does not exist" mode. This indicates
// that the operation should ensure the resource does not exist.
func (mc *moduleContext) DoesNotExist() bool {
	return mc.dne
}

// Tainted returns true if the operation is being run in "tainted" mode. This indicates that the
// operation should ensure the resource is re-created or re-configured even if it already exists
// and is correct. A check on a tainted resource should always return false.
func (mc *moduleContext) Tainted() bool {
	return mc.tainted
}

func (mc *moduleContext) Deadline() (deadline time.Time, ok bool) {
	return mc.ctx.Deadline()
}

func (mc *moduleContext) Done() <-chan struct{} {
	return mc.ctx.Done()
}

func (mc *moduleContext) Err() error {
	return mc.ctx.Err()
}

func (mc *moduleContext) Value(key any) any {
	return mc.ctx.Value(key)
}

const (
	undefinedInput inputType = iota
	stringInput
	boolInput
	numberInput
	unsignedNumberInput
	floatInput
	anyInput
)

// --8<-- [start:Input]
type Input interface {
	IsStatic() bool
	String() string
	Bool() bool
	Number() int64
	Float() float64
	Any() any
	Auto() (any, error)
	DependencyId() string
	OutputKey() string
}

// --8<-- [end:Input]

// moduleInput represents an input to a module. Input values may be simple scalar values or any
// value type output from a dependency.
type moduleInput struct {
	defaultType           inputType
	stringValue           string
	boolValue             bool
	numberValue           int64
	unsignedNumberValue   uint64
	floatValue            float64
	anyValue              any
	dependencyOutputValue *dependencyOutput
}

// IsStatic returns true if the input is a static value, false if it is only available at runtime.
func (m *moduleInput) IsStatic() bool {
	return m.dependencyOutputValue == nil
}

// Auto returns the value of the input in its native type. If the input is not static, an error is
// returned.
func (m *moduleInput) Auto() (any, error) {
	switch m.defaultType {
	case stringInput:
		return m.String(), nil
	case boolInput:
		return m.Bool(), nil
	case numberInput:
		return m.Number(), nil
	case unsignedNumberInput:
		return m.Number(), nil
	case floatInput:
		return m.Float(), nil
	case anyInput:
		return m.Any(), nil
	default:
		return nil, fmt.Errorf("unable to determine type")
	}
}

func (m *moduleInput) String() string {
	return m.stringValue
}

func (m *moduleInput) Bool() bool {
	return m.boolValue
}

func (m *moduleInput) Number() int64 {
	return m.numberValue
}

func (m *moduleInput) Float() float64 {
	return m.floatValue
}

func (m *moduleInput) Any() any {
	return m.anyValue
}

// DependencyId returns the ID of the operation that is a dependency for this input.
func (m *moduleInput) DependencyId() string {
	if m.dependencyOutputValue != nil {
		return m.dependencyOutputValue.OperationId
	}
	return ""
}

// OutputKey returns the output key from the dependency operation that provides the value for this
// input.
func (m *moduleInput) OutputKey() string {
	if m.dependencyOutputValue != nil {
		return m.dependencyOutputValue.Output
	}
	return ""
}

// NewInputFromValue creates a new module input from a static value. It detects the type and
// assigns it to the correct field.
func NewInputFromValue(value interface{}) Input {
	switch v := value.(type) {
	case string:
		return &moduleInput{stringValue: v, anyValue: v, defaultType: stringInput}
	case bool:
		return &moduleInput{boolValue: v, anyValue: v, defaultType: boolInput}
	case int:
		return &moduleInput{numberValue: int64(v), anyValue: v, defaultType: numberInput}
	case int64:
		return &moduleInput{numberValue: v, anyValue: v, defaultType: numberInput}
	case uint64:
		return &moduleInput{unsignedNumberValue: v, anyValue: v, defaultType: unsignedNumberInput}
	case float64:
		return &moduleInput{floatValue: v, anyValue: v, defaultType: floatInput}
	default:
		return &moduleInput{anyValue: value, defaultType: anyInput}
	}
}

// NewInputFromDep creates a new module input from a dependency output. This is used to reference
// the output of another operation as the input to this operation. These inputs are only available
// at runtime.
func NewInputFromDep(id string, output string) Input {
	return &moduleInput{
		dependencyOutputValue: &dependencyOutput{
			OperationId: id,
			Output:      output,
		},
	}
}

// dependencyOutput is used to represent an input that is provided by the output of another
// operation.
type dependencyOutput struct {
	OperationId string `yaml:"id"`
	Output      string `yaml:"output"`
}

// RegisterModule is used by modules to register themselves with the global module registry. Each
// module must provide its ID and a factory function that will be used to create new instances of
// the module. The module factory function should only create the module instance and not perform
// any setup or validation. It should also not need to return any error, setup and verification of
// the module will be done in the setup method.
func RegisterModule(module string, factory func() Module) {
	registeredModuleFactories[module] = factory
}

// GetRegisteredModules returns a map of all registered modules and their factory functions.
func GetRegisteredModules() map[string]func() Module {
	return registeredModuleFactories
}

// RegisterPathName is used by modules to register a friendly name for a path segment.
func RegisterPathName(path, name string) {
	registeredPathNames[path] = name
}

// GetRegisteredPathNames returns a map of all registered path names.
func GetRegisteredPathNames() map[string]string {
	return registeredPathNames
}

// NewModule creates a new module instance based on the provided operation. The operation has the
// module ID and this is used to create an instance of the Module interface.
func NewModule(op *Operation) (Module, error) {
	f, ok := registeredModuleFactories[op.Module]
	if ok && f != nil {
		m := f()
		if m == nil {
			return nil, fmt.Errorf("failed to create module: %v", op.Module)
		}
		return m, nil
	}
	return nil, fmt.Errorf("unknown module: %s", op.Module)
}

// InputsToContext is a helper function that takes a map of inputs and returns module context with
// the inputs set as if they were passed at runtime. This is available as a helper for module testing.
func InputsToContext(ctx context.Context, si map[string]Input, flags ...ModuleContextFlag) (
	mctx ModuleContext,
) {
	inputs := make(map[string]Input)
	for k, v := range si {
		if v.IsStatic() {
			inputs[k] = v
		}
	}
	mctx = &moduleContext{
		ctx:          ctx,
		inputValues:  inputs,
		outputValues: make(map[string]interface{}),
	}

	for _, flag := range flags {
		switch flag {
		case TaintedFlag:
			mctx.(*moduleContext).tainted = true
		case DoesNotExistFlag:
			mctx.(*moduleContext).dne = true
		}
	}

	return
}

// OpContext creates a ModuleContext from an Operation. This is available as an exported helper for
// testing and special cases where a ModuleContext needs to be created directly outside the normal
// execution flow.
//
// Use caution when using this function, as it will panic if attempting to access a module which
// has not been registered.
func OpContext(ctx context.Context, op *Operation) ModuleContext {
	return newModuleContext(ctx, op)
}
