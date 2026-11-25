package monitoring

type Options struct {
	YSF      bool `json:"ysf"`
	NXDN     bool `json:"nxdn"`
	EchoLink bool `json:"echolink"`
	DVSwitch bool `json:"dvSwitch"`
}

func All() Options {
	return Options{YSF: true, NXDN: true, EchoLink: true, DVSwitch: true}
}

func Resolve(options *Options) Options {
	if options == nil {
		return All()
	}
	return *options
}

func (o Options) Intersect(other Options) Options {
	return Options{
		YSF: o.YSF && other.YSF, NXDN: o.NXDN && other.NXDN,
		EchoLink: o.EchoLink && other.EchoLink, DVSwitch: o.DVSwitch && other.DVSwitch,
	}
}
