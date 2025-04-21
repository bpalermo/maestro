package types

type ClusterName string

func (c ClusterName) ToString() string {
	return string(c)
}
