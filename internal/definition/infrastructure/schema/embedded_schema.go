package schema

import _ "embed"

//go:embed create_execution_v1.schema.json
var inputSchema []byte

//go:embed create_execution_v2.schema.json
var inputSchemaV2 []byte
