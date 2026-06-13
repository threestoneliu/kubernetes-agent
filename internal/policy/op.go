package policy

type OperationInfo interface {
	Action() string
	Resource() string
	Namespace() string
	Kind() string
	Manifest() map[string]any
}
