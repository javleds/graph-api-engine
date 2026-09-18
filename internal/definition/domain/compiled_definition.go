package domain

type CompiledDefinition struct {
	Version int
	API     APIConnection
	Roots   []CompiledCall
	Webhook *Webhook
}
