package observability

type Noop struct{}

func (Noop) Incr(string, map[string]string)          {}
func (Noop) Add(string, float64, map[string]string)   {}
func (Noop) Set(string, float64, map[string]string)   {}
func (Noop) Observe(string, float64, map[string]string) {}
