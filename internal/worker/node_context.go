package worker

// NodeContext contains frozen ancestor values and a structural traversal key.
type NodeContext struct {
	Values map[string]any
	Path   string
	Order  []int
	Page   *int64
}

func (n NodeContext) child(name string, index int, values map[string]any) NodeContext {
	return NodeContext{Values: values, Path: n.Path + "." + name, Order: appendOrder(n.Order, index)}
}
func appendOrder(base []int, items ...int) []int {
	result := make([]int, 0, len(base)+len(items))
	result = append(result, base...)
	return append(result, items...)
}
