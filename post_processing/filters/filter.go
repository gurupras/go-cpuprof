package filters

import "github.com/gurupras/go_cpuprof"

type LineFilter func(line string) bool
type LoglineFilter func(logline *cpuprof.Logline) bool

type Filter struct {
	filterFuncs []LoglineFilter
}

func New() *Filter {
	f := new(Filter)
	f.filterFuncs = make([]LoglineFilter, 0)
	return f
}

func (f *Filter) AddFilter(filter LoglineFilter) {
	f.filterFuncs = append(f.filterFuncs, filter)
}

func (f *Filter) AsLineFilterArray() []LoglineFilter {
	return f.filterFuncs
}

func (f *Filter) Apply(line string) {
	logline := cpuprof.ParseLogline(line)
	if logline == nil {
		return
	}
	for _, ffunc := range f.filterFuncs {
		ffunc(logline)
	}
}
