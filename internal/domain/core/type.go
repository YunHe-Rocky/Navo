package core

type Type string

const (
	TypeMihomo  Type = "mihomo"
	TypeXray    Type = "xray"
	TypeSingBox Type = "sing-box"
)

func (t Type) Valid() bool {
	switch t {
	case TypeMihomo, TypeXray, TypeSingBox:
		return true
	default:
		return false
	}
}

func (t Type) String() string {
	return string(t)
}

func All() []Type {
	return []Type{TypeMihomo, TypeXray, TypeSingBox}
}
