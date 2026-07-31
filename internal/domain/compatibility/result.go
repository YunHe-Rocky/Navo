package compatibility

type Level string

const (
	LevelSupported                Level = "supported"
	LevelSupportedWithLimitations Level = "supported_with_limitations"
	LevelUnsupported              Level = "unsupported"
)

type Reason struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Result struct {
	Supported bool      `json:"supported"`
	Level     Level     `json:"level"`
	Reasons   []Reason  `json:"reasons,omitempty"`
	Warnings  []Warning `json:"warnings,omitempty"`
}

func Supported() Result {
	return Result{Supported: true, Level: LevelSupported}
}

func Limited(warnings ...Warning) Result {
	return Result{
		Supported: true,
		Level:     LevelSupportedWithLimitations,
		Warnings:  append([]Warning(nil), warnings...),
	}
}

func Unsupported(reasons ...Reason) Result {
	return Result{
		Supported: false,
		Level:     LevelUnsupported,
		Reasons:   append([]Reason(nil), reasons...),
	}
}

func (r Result) Valid() bool {
	switch r.Level {
	case LevelSupported:
		return r.Supported && len(r.Reasons) == 0
	case LevelSupportedWithLimitations:
		return r.Supported && len(r.Reasons) == 0 && len(r.Warnings) > 0
	case LevelUnsupported:
		return !r.Supported && len(r.Reasons) > 0
	default:
		return false
	}
}
