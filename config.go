package blackstart

import (
	"fmt"
	"reflect"

	"github.com/jessevdk/go-flags"
)

var LogOutputEnv = getConfigEnv("LogOutput")
var K8sNamespaceEnv = getConfigEnv("KubeNamespace")

type RuntimeConfig struct {
	LogOutput     string `long:"log-output" env:"BLACKSTART_LOG_OUTPUT" description:"Logging output file name" default:""`
	LogFormat     string `long:"log-format" env:"BLACKSTART_LOG_FORMAT" description:"Logging format (json, text)" default:"text"`
	LogLevel      string `long:"log-level" env:"BLACKSTART_LOG_LEVEL" description:"Logging level" default:"info"`
	WorkflowFile  string `short:"f" long:"workflow-file" env:"BLACKSTART_WORKFLOW_FILE" description:"Path to the workflow file" required:"false"`
	KubeNamespace string `short:"n" long:"k8s-namespace" env:"BLACKSTART_K8S_NAMESPACE" description:"Kubernetes namespace(s) to read the workflow from" default:""`
}

func ReadConfig() (*RuntimeConfig, error) {
	config := &RuntimeConfig{}
	parser := flags.NewParser(config, flags.Default|flags.IgnoreUnknown)
	_, err := parser.Parse()
	if err != nil {
		return nil, err
	}

	return config, nil
}

func getConfigEnv(name string) string {
	c := RuntimeConfig{}
	typ := reflect.TypeOf(c)

	field, found := typ.FieldByName(name)
	if !found {
		panic(fmt.Sprintf("configuration field %s not found", name))
	}
	return field.Tag.Get("env")
}
