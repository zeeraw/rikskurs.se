package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/zeeraw/riksbank/cli/flags"
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
	r.HandleFunc("/exchange/rate/{base:[a-zA-Z]{3}}/{counter:[a-zA-Z]{3}}/{date}", exchangeRateHandler(rb))

	server := &http.Server{
		Handler: r,
	}
	if production {
		certManager := autocert.Manager{
			Prompt: autocert.AcceptTOS,
			Cache:  autocert.DirCache("certs"),
		}
		server.Addr = ":443"
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
		fmt.Fprintf(tw, "latest exchange rate for a currency pair\t /exchange/rate/{base}/{counter}\t %s/exchange/rate/sek/nok\n", r.Host)
		fmt.Fprintf(tw, "exchange rate for currency pair on specific date\t /exchange/rate/{base}/{counter}/{date}\t %s/exchange/rate/sek/nok/2019-01-01\n", r.Host)
		w.WriteHeader(200)
	}
}

var (
	errNeedBaseCurrency          = errors.New("need base currency")
	errNeedCounterCurrency       = errors.New("need counter currency")
	errNoCurrencyDataForPeriod   = errors.New("no data for currencies in that period")
	errNoConversionRateForPeriod = errors.New("no conversion rate for that period")
)

func exchangeRateHandler(rb *riksbank.Riksbank) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		vars := mux.Vars(r)
		var date time.Time
		if vars["date"] == "" {
			date = time.Date(2019, 1, 6, 0, 0, 0, 0, time.UTC)
		} else {
			t, err := flags.ParseDate(vars["date"])
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
			date = t
		}
		if vars["base"] == "" {
			http.Error(w, errNeedBaseCurrency.Error(), http.StatusUnprocessableEntity)
			return
		}
		base := currency.Parse(vars["base"])
		if vars["counter"] == "" {
			http.Error(w, errNeedCounterCurrency.Error(), http.StatusUnprocessableEntity)
			return
		}
		counter := currency.Parse(vars["counter"])
		rate, err := rateForDate(r.Context(), rb, base, counter, date)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		fmt.Fprintf(w, "%f", rate)
	}
}

func rateForDate(ctx context.Context, rb *riksbank.Riksbank, base, counter currency.Currency, date time.Time) (rate float64, err error) {
	res, err := rb.ExchangeRates(ctx, &riksbank.ExchangeRatesRequest{
		CurrencyPairs: []currency.Pair{
			currency.Pair{
				Base:    base,
				Counter: counter,
			},
		},
		AggregateMethod: riksbank.Daily,
		From:            date.AddDate(0, 0, -7),
		To:              date,
	})
	if err != nil {
		return rate, err
	}
	exchangeRates := riksbank.ExchangeRates{}
	for _, er := range res.ExchangeRates {
		if er.Base == base && er.Counter == counter {
			exchangeRates = append(exchangeRates, er)
		}
	}
	sort.Slice(exchangeRates, func(i int, j int) bool {
		return exchangeRates[j].Date.UnixNano() < exchangeRates[i].Date.UnixNano()
	})
	if len(exchangeRates) < 1 {
		return rate, errNoCurrencyDataForPeriod
	}
	value := exchangeRates[0].Value
	if value == nil {
		return rate, errNoConversionRateForPeriod
	}
	return *value, nil
}

func exchangeHandler(rb *riksbank.Riksbank) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
	}
}
