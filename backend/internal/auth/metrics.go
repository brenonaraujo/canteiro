package auth

import "github.com/prometheus/client_golang/prometheus"

var (
	loginTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_login_total",
		Help: "Google login outcomes (ok, denied, error)",
	}, []string{"result"})
	logoutTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "auth_logout_total",
		Help: "Session logout count",
	})
	deactivateTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "account_deactivate_total",
		Help: "Account deactivations",
	})
)

func init() {
	prometheus.MustRegister(loginTotal, logoutTotal, deactivateTotal)
}

func incLogin(result string) {
	loginTotal.WithLabelValues(result).Inc()
}

func incLogout() {
	logoutTotal.Inc()
}

func incDeactivate() {
	deactivateTotal.Inc()
}
