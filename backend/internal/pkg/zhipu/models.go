package zhipu

const (
	Platform         = "zhipu"
	DefaultTestModel = "glm-5.2"
)

var defaultModelIDs = []string{
	"glm-5.2",
	"glm-5.2[1m]",
	"glm-5.1",
	"glm-5-turbo",
	"glm-5",
	"glm-4.7",
	"glm-4.7-flashx",
	"glm-4.7-flash",
	"glm-4.5-air",
}

func DefaultModelIDs() []string {
	return append([]string(nil), defaultModelIDs...)
}
