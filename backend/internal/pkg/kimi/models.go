package kimi

const (
	Platform           = "kimi"
	CodeK3Model        = "k3"
	CodeModel          = "kimi-for-coding"
	CodeHighSpeedModel = "kimi-for-coding-highspeed"
)

var codeModelIDs = []string{
	CodeK3Model,
	CodeModel,
	CodeHighSpeedModel,
}

var apiModelIDs = []string{
	"kimi-k3",
	"kimi-k2.7-code",
	"kimi-k2.7-code-highspeed",
	"kimi-k2.6",
	"kimi-k2.5",
}

func DefaultModelIDs() []string {
	models := make([]string, 0, len(codeModelIDs)+len(apiModelIDs))
	models = append(models, codeModelIDs...)
	models = append(models, apiModelIDs...)
	return models
}

func CodeModelIDs() []string {
	return append([]string(nil), codeModelIDs...)
}

func APIModelIDs() []string {
	return append([]string(nil), apiModelIDs...)
}
