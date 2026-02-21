package observability

type Counter interface {
	Incr(name string, tags map[string]string)
	Add(name string, value float64, tags map[string]string)
}

type Gauge interface {
	Set(name string, value float64, tags map[string]string)
}

type Histogram interface {
	Observe(name string, value float64, tags map[string]string)
}
