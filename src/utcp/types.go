package utcp

// UtcpManual represents the authoritative manual returned during discovery.
type UtcpManual struct {
	UtcpVersion   string `json:"utcp_version"`   // e.g. "1.0.1"
	ManualVersion string `json:"manual_version"` // e.g. "1.0.0"
	Tools         []Tool `json:"tools"`
}

// Tool represents an individual capability exposed by Searqon.
type Tool struct {
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Inputs           ToolInputSchema   `json:"inputs"`
	Outputs          *ToolOutputSchema `json:"outputs,omitempty"`
	Tags             []string          `json:"tags,omitempty"`
	ToolCallTemplate CallTemplate      `json:"tool_call_template"`
}

// ToolInputSchema defines standard JSON Schema Draft-07 input parameters.
type ToolInputSchema struct {
	Type       string                       `json:"type"` // "object"
	Properties map[string]ToolParamProperty `json:"properties"`
	Required   []string                     `json:"required,omitempty"`
}

// ToolParamProperty defines a property inside a ToolInputSchema.
type ToolParamProperty struct {
	Type        string   `json:"type"` // "string", "integer", "boolean", "array", "object"
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Default     any      `json:"default,omitempty"`
}

// ToolOutputSchema defines the schema for tool output.
type ToolOutputSchema struct {
	Type       string                 `json:"type"` // e.g. "object"
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// CallTemplate defines the transport binding for invoking the tool.
type CallTemplate struct {
	CallTemplateType string            `json:"call_template_type"` // "http" | "sse"
	URL              string            `json:"url"`
	HTTPMethod       string            `json:"http_method,omitempty"`  // "POST" | "GET"
	ContentType      string            `json:"content_type,omitempty"` // "application/json"
	Headers          map[string]string `json:"headers,omitempty"`
}
