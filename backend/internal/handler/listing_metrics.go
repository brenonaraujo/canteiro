package handler

import "github.com/prometheus/client_golang/prometheus"

var catalogSearchTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "catalog_search_total",
	Help: "Public catalog search outcomes (ok, empty, error)",
}, []string{"result"})

func init() {
	prometheus.MustRegister(catalogSearchTotal)
}

func incCatalogSearch(result string) {
	catalogSearchTotal.WithLabelValues(result).Inc()
}

func catalogSearchResult(total int, err error) string {
	if err != nil {
		return "error"
	}
	if total == 0 {
		return "empty"
	}
	return "ok"
}
