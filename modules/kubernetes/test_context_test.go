package kubernetes

import "github.com/pezops/blackstart"

// capturingModuleContext records module outputs while preserving normal context behavior.
type capturingModuleContext struct {
	blackstart.ModuleContext
	outputs map[string]interface{}
}

// Output records the output value and delegates to the wrapped ModuleContext.
func (c *capturingModuleContext) Output(key string, value interface{}) error {
	if c.outputs == nil {
		c.outputs = map[string]interface{}{}
	}
	c.outputs[key] = value
	return c.ModuleContext.Output(key, value)
}
