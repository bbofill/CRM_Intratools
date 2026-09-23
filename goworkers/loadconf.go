package goworkers

// set&parse main config file
func SetConfigFile(configFile string) {
	mC = readConfigFile(configFile)
}

// set&parse random cookies (tmp)
func SetMainCookie() {
	cOoKiEnAmE = "CRMINTRATOOLS"
}

// set&parse module conf obj
func SetModuleConf() {
	mOdUlEcOnF = Module{
		mC.Module0Name:  {mC.ModuleConfig.Module0ServerPort, mC.ModuleConfig.Module0ApiKey, mC.ModuleConfig.Module0StartCommand, mC.ModuleConfig.Module0StatusEnabled},
		mC.Module1Name:  {mC.ModuleConfig.Module1ServerPort, mC.ModuleConfig.Module1ApiKey, mC.ModuleConfig.Module1StartCommand, mC.ModuleConfig.Module1StatusEnabled},
		mC.Module2Name:  {mC.ModuleConfig.Module2ServerPort, mC.ModuleConfig.Module2ApiKey, mC.ModuleConfig.Module2StartCommand, mC.ModuleConfig.Module2StatusEnabled},
		mC.Module3Name:  {mC.ModuleConfig.Module3ServerPort, mC.ModuleConfig.Module3ApiKey, mC.ModuleConfig.Module3StartCommand, mC.ModuleConfig.Module3StatusEnabled},
		mC.Module4Name:  {mC.ModuleConfig.Module4ServerPort, mC.ModuleConfig.Module4ApiKey, mC.ModuleConfig.Module4StartCommand, mC.ModuleConfig.Module4StatusEnabled},
		mC.Module5Name:  {mC.ModuleConfig.Module5ServerPort, mC.ModuleConfig.Module5ApiKey, mC.ModuleConfig.Module5StartCommand, mC.ModuleConfig.Module5StatusEnabled},
		mC.Module6Name:  {mC.ModuleConfig.Module6ServerPort, mC.ModuleConfig.Module6ApiKey, mC.ModuleConfig.Module6StartCommand, mC.ModuleConfig.Module6StatusEnabled},
		mC.Module7Name:  {mC.ModuleConfig.Module7ServerPort, mC.ModuleConfig.Module7ApiKey, mC.ModuleConfig.Module7StartCommand, mC.ModuleConfig.Module7StatusEnabled},
		mC.Module8Name:  {mC.ModuleConfig.Module8ServerPort, mC.ModuleConfig.Module8ApiKey, mC.ModuleConfig.Module8StartCommand, mC.ModuleConfig.Module8StatusEnabled},
		mC.Module9Name:  {mC.ModuleConfig.Module9ServerPort, mC.ModuleConfig.Module9ApiKey, mC.ModuleConfig.Module9StartCommand, mC.ModuleConfig.Module9StatusEnabled},
		mC.Module10Name: {mC.ModuleConfig.Module10ServerPort, mC.ModuleConfig.Module10ApiKey, mC.ModuleConfig.Module10StartCommand, mC.ModuleConfig.Module10StatusEnabled},
		mC.Module11Name: {mC.ModuleConfig.Module11ServerPort, mC.ModuleConfig.Module11ApiKey, mC.ModuleConfig.Module11StartCommand, mC.ModuleConfig.Module11StatusEnabled},
	}
}
