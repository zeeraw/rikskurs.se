package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"text/tabwriter"
	"time"

	"github.com/zeeraw/riksbank/currency"
	"golang.org/x/crypto/acme/autocert"

	"github.com/gorilla/mux"
	"github.com/zeeraw/riksbank"
)

func main() {
	rb := riksbank.New(riksbank.Config{})
	r := mux.NewRouter()

	var production bool
	flag.BoolVar(&production, "production", false, "use this flag to run the application in production mode")
	flag.Parse()

	r.HandleFunc("/", homeHandler())
	r.HandleFunc("/exchange/rate/{base:[a-zA-Z]{3}}/{counter:[a-zA-Z]{3}}", exchangeRateHandler(rb))

	server := &http.Server{
		Handler: r,
	}
	if production {
		certManager := autocert.Manager{
			Prompt: autocert.AcceptTOS,
			Cache:  autocert.DirCache("certs"),
		}
		server.Addr = ":433"
		server.TLSConfig = &tls.Config{
			GetCertificate: certManager.GetCertificate,
		}
		go http.ListenAndServe(":80", certManager.HTTPHandler(nil))
		server.ListenAndServeTLS("", "")
		return
	}
	server.Addr = ":8080"
	server.ListenAndServeTLS("certs/local.rikskurs.se.pem", "certs/local.rikskurs.se-key.pem")
}

func homeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		tw := tabwriter.NewWriter(w, 0, 0, 0, ' ', 0)
		defer tw.Flush()
		fmt.Fprintf(tw, "description\t url\t example\n")
		fmt.Fprintf(tw, "currency exchange rate\t /exchange/rate/{base}/{counter}\t %s/exchange/rate/sek/nok\n", r.Host)
		w.WriteHeader(200)
	}
}

func exchangeRateHandler(rb *riksbank.Riksbank) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		vars := mux.Vars(r)
		if vars["base"] == "" {
			http.Error(w, "need base currency", http.StatusUnprocessableEntity)
			return
		}
		base := currency.Parse(vars["base"])
		if vars["counter"] == "" {
			http.Error(w, "need counter currency", http.StatusUnprocessableEntity)
			return
		}
		counter := currency.Parse(vars["counter"])
		res, err := rb.ExchangeRates(r.Context(), &riksbank.ExchangeRatesRequest{
			CurrencyPairs: []currency.Pair{
				currency.Pair{
					Base:    base,
					Counter: counter,
				},
			},
			AggregateMethod: riksbank.Daily,
			From:            time.Now(),
			To:              time.Now(),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var value *float64
		for _, er := range res.ExchangeRates {
			if er.Base == base && er.Counter == counter {
				value = er.Value
				break
			}
		}
		if value == nil {
			http.Error(w, "no data for currencies in that period", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "%f", *value)
	}
}

func exchangeHandler(rb *riksbank.Riksbank) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
	}
}
