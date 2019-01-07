package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"sort"
	"strconv"
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
	r.HandleFunc(`/exchange/{value:\d+.\d+}/{base:[a-zA-Z]{3}}/{counter:[a-zA-Z]{3}}`, exchangeHandler(rb))
	r.HandleFunc(`/exchange/{value:\d+}/{base:[a-zA-Z]{3}}/{counter:[a-zA-Z]{3}}`, exchangeHandler(rb))
	r.HandleFunc(`/exchange/{value:\d+.\d+}/{base:[a-zA-Z]{3}}/{counter:[a-zA-Z]{3}}/{date}`, exchangeHandler(rb))
	r.HandleFunc(`/exchange/{value:\d+}/{base:[a-zA-Z]{3}}/{counter:[a-zA-Z]{3}}/{date}`, exchangeHandler(rb))

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
		endpoints := []struct {
			desc    string
			url     string
			example string
		}{
			{"latest exchange rate for a currency pair", "/exchange/rate/{base}/{counter}", "/exchange/rate/sek/nok"},
			{"exchange rate for currency pair on a specific date", "/exchange/rate/{base}/{counter}/{date}", "/exchange/rate/sek/nok/2019-01-01"},
			{"convert currency at the latest exchange rate", "/exchange/{value}/{base}/{counter}", "/exchange/1200.5/sek/nok"},
			{"convert currency at the exchange rate of a specific date", "/exchange/{value}/{base}/{counter}/{date}", "/exchange/1200.5/sek/nok/2019-01-01"},
		}
		fmt.Fprintf(w, "exchange and interest rate http api with only plain text response values (data sourced from riksbank.se)\n\n")
		fmt.Fprintf(tw, "description\t url\t example\n")
		for _, ep := range endpoints {
			fmt.Fprintf(tw, "%s\t %s\t %s%s\n", ep.desc, ep.url, r.Host, ep.example)
		}
		w.WriteHeader(200)
	}
}

var (
	errNeedBaseCurrency          = errors.New("need base currency")
	errNeedCounterCurrency       = errors.New("need counter currency")
	errNoCurrencyDataForPeriod   = errors.New("no data for currencies in that period")
	errNoConversionRateForPeriod = errors.New("no conversion rate for that period")
)

func parseExchangeParams(r *http.Request) (base, counter currency.Currency, date time.Time, err error) {
	vars := mux.Vars(r)
	if vars["date"] == "" {
		date = time.Date(2019, 1, 6, 0, 0, 0, 0, time.UTC)
	} else {
		t, err := flags.ParseDate(vars["date"])
		if err != nil {
			return base, counter, date, err
		}
		date = t
	}
	if vars["base"] == "" {
		return base, counter, date, errNeedBaseCurrency
	}
	base = currency.Parse(vars["base"])
	if vars["counter"] == "" {
		return base, counter, date, errNeedCounterCurrency
	}
	counter = currency.Parse(vars["counter"])
	return base, counter, date, nil
}

func parseValueParams(r *http.Request) (f float64, err error) {
	vars := mux.Vars(r)
	if vars["value"] == "" {
		return f, errors.New("no value provided")
	}
	f, err = strconv.ParseFloat(vars["value"], 64)
	return f, err
}

func exchangeHandler(rb *riksbank.Riksbank) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		value, err := parseValueParams(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		base, counter, date, err := parseExchangeParams(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		rate, err := rateForDate(r.Context(), rb, base, counter, date)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		fmt.Fprintf(w, "%f", rate*value)
	}
}

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
