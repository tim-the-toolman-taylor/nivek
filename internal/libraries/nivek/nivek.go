package nivek

var engine NivekService

func setEngine(service NivekService) {
	engine = service
}

func GetEngine() NivekService {
	return engine
}
