package blackstart

import (
	"fmt"
	"reflect"

	"github.com/jessevdk/go-flags"
)

var LogOutputEnv = getConfigEnv("LogOutput")
var K8sNamespaceEnv = getConfigEnv("KubeNamespace")
var RuntimeModeEnv = getConfigEnv("RuntimeMode")

type RuntimeConfig struct {
	Version                    bool   `short:"v" long:"version" description:"Show version information"`
	LogOutput                  string `long:"log-output" env:"BLACKSTART_LOG_OUTPUT" description:"Logging output file name" default:""`
	LogFormat                  string `long:"log-format" env:"BLACKSTART_LOG_FORMAT" description:"Logging format (json, text)" default:"text"`
	LogLevel                   string `long:"log-level" env:"BLACKSTART_LOG_LEVEL" description:"Logging level" default:"info"`
	LogLevelKey                string `long:"log-level-key" env:"BLACKSTART_LOG_LEVEL_KEY" description:"JSON logging key name for level/severity" default:"level"`
	LogMessageKey              string `long:"log-message-key" env:"BLACKSTART_LOG_MESSAGE_KEY" description:"JSON logging key name for message/event" default:"msg"`
	WorkflowFile               string `short:"f" long:"workflow-file" env:"BLACKSTART_WORKFLOW_FILE" description:"Path to the workflow file" required:"false"`
	KubeNamespace              string `short:"n" long:"k8s-namespace" env:"BLACKSTART_K8S_NAMESPACE" description:"Kubernetes namespace(s) to read the workflow from" default:""`
	RuntimeMode                string `long:"runtime-mode" env:"BLACKSTART_RUNTIME_MODE" description:"Runtime mode when reading workflows from Kubernetes (controller, once)" default:"controller"`
	MaxParallelReconciliations int    `long:"max-parallel-reconciliations" env:"BLACKSTART_MAX_PARALLEL_RECONCILIATIONS" description:"Maximum number of workflows to reconcile in parallel" default:"4"`
	ControllerResyncInterval   string `long:"controller-resync-interval" env:"BLACKSTART_CONTROLLER_RESYNC_INTERVAL" description:"How often to refresh watched workflows from Kubernetes" default:"15s"`
	QueueWaitWarningThreshold  string `long:"queue-wait-warning-threshold" env:"BLACKSTART_QUEUE_WAIT_WARNING_THRESHOLD" description:"Warn when a queued workflow waits longer than this duration before running" default:"30s"`
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
