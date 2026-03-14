package fix

var validVersions =  map[string]bool{
	"1.1": true,
	"4.0": true,
	"4.1": true,
	"4.2": true,
	"4.3": true,
	"4.4": true,
	"5.0": true,
}

func IsVersionValid(fixVersion string) bool {
	if _, ok := validVersions[fixVersion]; ok {
		return true
	}
	return false
}
