package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/gorilla/mux"
	"github.com/zeeraw/riksbank"
	"github.com/zeeraw/riksbank/currency"
	"github.com/zeeraw/riksbank/date"
)

var (
	errNeedBaseCurrency          = errors.New("need base currency")
	errNeedCounterCurrency       = errors.New("need counter currency")
	errNoCurrencyDataForPeriod   = errors.New("no data for currencies in that period")
	errNoConversionRateForPeriod = errors.New("no conversion rate for that period")
)

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

func parseDateParam(r *http.Request) (day time.Time, err error) {
	vars := mux.Vars(r)
	if vars["date"] == "" {
		day = time.Now()
	} else {
		t, err := date.Parse(vars["date"])
		if err != nil {
			return day, err
		}
		day = t
	}
	return day, nil
}

func parseExchangeParams(r *http.Request) (base, counter currency.Currency, day time.Time, err error) {
	vars := mux.Vars(r)
	day, err = parseDateParam(r)
	if err != nil {
		return base, counter, day, err
	}
	if vars["base"] == "" {
		return base, counter, day, errNeedBaseCurrency
	}
	base = currency.Parse(vars["base"])
	if vars["counter"] == "" {
		return base, counter, day, errNeedCounterCurrency
	}
	counter = currency.Parse(vars["counter"])
	return base, counter, day, nil
}

func parseValueParams(r *http.Request) (f float64, err error) {
	vars := mux.Vars(r)
	if vars["value"] == "" {
		return f, errors.New("no value provided")
	}
	f, err = strconv.ParseFloat(vars["value"], 64)
	return f, err
}

func homeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		tw := tabwriter.NewWriter(w, 0, 0, 0, ' ', 0)
		defer tw.Flush()
		fmt.Fprintf(w, "exchange and interest rate http api with only plain text response values (data sourced from riksbank.se)\n\n")
		fmt.Fprintf(tw, "description\t url\t example\n")
		for _, ep := range endpoints {
			fmt.Fprintf(tw, "%s\t %s\t %s%s\n", ep.desc, ep.url, r.Host, ep.example)
		}
		return
	}
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
			return
		}
		fmt.Fprintf(w, "%f", rate*value)
	}
}

func exchangeRateHandler(rb *riksbank.Riksbank) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		base, counter, date, err := parseExchangeParams(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		rate, err := rateForDate(r.Context(), rb, base, counter, date)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "%f", rate)
	}
}

func dayHandler(rb *riksbank.Riksbank) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		day, err := parseDateParam(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		res, err := rb.Days(r.Context(), &riksbank.DaysRequest{
			From: day,
			To:   day,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "%v", res.Days[0].IsBankDay)
	}
}
