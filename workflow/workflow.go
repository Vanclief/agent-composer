package workflow

import (
	"encoding/json"
	"strings"

	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/runtime/harnesses"
	runtimetypes "github.com/vanclief/agent-composer/runtime/types"
)

type Blueprint struct {
	Workflow        WorkflowSpec          `yaml:"workflow"`
	Schemas         map[string]SchemaSpec `yaml:"schemas"`
	Nodes           map[string]NodeSpec   `yaml:"nodes"`
	Flow            FlowSpec              `yaml:"flow"`
	SourcePath      string                `yaml:"-"`
	NodeInputOrder  map[string][]string   `yaml:"-"`
	NodeOutputOrder map[string][]string   `yaml:"-"`
}

type WorkflowSpec struct {
	ID          string                        `yaml:"id"`
	Version     string                        `yaml:"version"`
	Description string                        `yaml:"description"`
	Inputs      map[string]string             `yaml:"inputs"`
	Outputs     map[string]WorkflowOutputSpec `yaml:"outputs"`
}

type WorkflowOutputSpec struct {
	Schema string `yaml:"schema"`
	From   string `yaml:"from"`
}

type WorkflowSummary struct {
	ID          string            `json:"id"`
	Description string            `json:"description,omitempty"`
	Inputs      map[string]string `json:"inputs"`
	Outputs     map[string]string `json:"outputs"`
}

type SchemaSpec struct {
	Type        string                `yaml:"type"`
	SchemaRef   string                `yaml:"schema_ref"`
	Properties  map[string]SchemaSpec `yaml:"properties"`
	Items       *SchemaSpec           `yaml:"items"`
	Enum        []any                 `yaml:"enum"`
	Optional    bool                  `yaml:"optional"`
	Description string                `yaml:"description"`
	Nullable    bool                  `yaml:"nullable"`
}

type NodeSpec struct {
	Kind          string              `yaml:"kind"`
	WorkflowID    string              `yaml:"workflow_id"`
	Operation     string              `yaml:"operation"`
	Executes      string              `yaml:"executes"`
	Over          string              `yaml:"over"`
	Updates       string              `yaml:"updates"`
	BreaksOn      string              `yaml:"breaks_on"`
	MaxIterations int                 `yaml:"max_iterations"`
	RoutesOn      string              `yaml:"routes_on"`
	WhenTrue      string              `yaml:"when_true"`
	WhenFalse     string              `yaml:"when_false"`
	Inputs        map[string]string   `yaml:"inputs"`
	Outputs       map[string]string   `yaml:"outputs"`
	Config        InferenceNodeConfig `yaml:"config"`
}

type InferenceNodeConfig struct {
	Harness     map[string]any `yaml:"harness"`
	Instruction string         `yaml:"instruction"`
}

type FlowSpec struct {
	Instances map[string]InstanceSpec `yaml:"instances"`
}

type InstanceSpec struct {
	Node   string            `yaml:"node"`
	Inputs map[string]string `yaml:"inputs"`
}

type Snapshot struct {
	WorkflowID      string
	WorkflowVersion string
	Description     string
	Inputs          map[string]Port
	Outputs         map[string]OutputBinding
	Nodes           map[string]NodeSnapshot
	Order           []string
}

type Port struct {
	Name    string
	TypeRef string
	Schema  map[string]any
}

type OutputBinding struct {
	Name   string
	Schema map[string]any
	From   Binding
}

type NodeSnapshot struct {
	InstanceID                string
	NodeName                  string
	Kind                      string
	Operation                 string
	Executes                  string
	Over                      string
	Updates                   string
	BreaksOn                  string
	MaxIterations             int
	RoutesOn                  string
	WhenTrue                  string
	WhenFalse                 string
	Instruction               string
	Harness                   agent.Harness `json:"Harness,omitempty"`
	Model                     string
	ReasoningEffort           runtimetypes.ReasoningEffort `json:"ReasoningEffort,omitempty"`
	HarnessConfig             json.RawMessage
	Inputs                    map[string]Port
	InputOrder                []string
	InputBindings             map[string]Binding
	Outputs                   map[string]Port
	Workflow                  *Snapshot
	OutputName                string
	OutputSchema              map[string]any
	StructuredOutputSchema    map[string]any
	StructuredOutputSchemaRaw json.RawMessage
	WrapStructuredOutput      bool
	LoopTarget                *NodeSnapshot
	WhileTarget               *WhileTargetSnapshot
	TrueTarget                *NodeSnapshot
	FalseTarget               *NodeSnapshot
}

type WhileTargetSnapshot struct {
	InstanceID                string
	NodeName                  string
	Instruction               string
	Harness                   agent.Harness `json:"Harness,omitempty"`
	Model                     string
	ReasoningEffort           runtimetypes.ReasoningEffort `json:"ReasoningEffort,omitempty"`
	HarnessConfig             json.RawMessage
	Inputs                    map[string]Port
	Workflow                  *Snapshot
	UpdateOutputName          string
	UpdateOutputSchema        map[string]any
	BreakOutputName           string
	StructuredOutputSchema    map[string]any
	StructuredOutputSchemaRaw json.RawMessage
}

type Binding struct {
	Kind          BindingKind
	WorkflowInput string
	InstanceID    string
	OutputName    string
}

type BindingKind string

const (
	BindingKindWorkflowInput BindingKind = "workflow_input"
	BindingKindInstance      BindingKind = "instance"
)

type Executor struct {
	NewHarness func(kind agent.Harness) (harnesses.Harness, error)
	Recorder   ExecutionRecorder
	ShellRoot  string
}

type workflowResolver struct {
	byID       map[string]*Blueprint
	searchDirs []string
}

type compiledWorkflow struct {
	Nodes        map[string]NodeSnapshot
	Dependencies map[string][]string
	Outputs      map[string]Binding
}

type compiledNodeShape struct {
	Inputs                    map[string]Port
	InputOrder                []string
	Outputs                   map[string]Port
	OutputName                string
	OutputSchema              map[string]any
	StructuredOutputSchema    map[string]any
	StructuredOutputSchemaRaw json.RawMessage
	WrapStructuredOutput      bool
	Harness                   agent.Harness
	Model                     string
	ReasoningEffort           runtimetypes.ReasoningEffort
	HarnessConfig             json.RawMessage
	Instruction               string
}

func NewExecutor(shellRoot string) *Executor {
	return &Executor{
		NewHarness: harnesses.New,
		ShellRoot:  strings.TrimSpace(shellRoot),
	}
}
