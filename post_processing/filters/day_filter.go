package filters

import "github.com/gurupras/go_cpuprof"

// Filter to provide callback every 24 hours
// Can be tweaked to provide callback after arbitrary durations of time

type DayFilter struct {
	*Filter
	Callback        func(line string)
	FilterFunc      LoglineFilter
	DayStartLogline *cpuprof.Logline
}

func NewDayFilter(filter *Filter) *DayFilter {
	df := new(DayFilter)
	df.Filter = filter
	df.Callback = nil

	filterFunc := func(logline *cpuprof.Logline) bool {
		// Always returns true. This is only used for the callback
		if df.DayStartLogline == nil {
			df.DayStartLogline = logline
			goto done
		} else {
			if logline.Datetime.YearDay() != df.DayStartLogline.Datetime.YearDay() && df.Callback != nil {
				df.Callback(logline.Line)
				df.DayStartLogline = logline
			}
		}
	done:
		return true
	}
	df.FilterFunc = filterFunc
	filter.AddFilter(filterFunc)
	return df
}
