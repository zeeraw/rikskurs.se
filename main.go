package main

import (
	"crypto/tls"
	"flag"
	"net/http"

	"golang.org/x/crypto/acme/autocert"

	"github.com/gorilla/mux"
	"github.com/zeeraw/riksbank"
)

var (
	endpoints = []struct {
		desc    string
		url     string
		example string
	}{
		{"latest exchange rate for a currency pair", "/exchange/rate/{base}/{counter}", "/exchange/rate/sek/nok"},
		{"exchange rate for currency pair on a specific date", "/exchange/rate/{base}/{counter}/{date}", "/exchange/rate/sek/nok/2019-01-01"},
		{"convert currency at the latest exchange rate", "/exchange/{value}/{base}/{counter}", "/exchange/1200.5/sek/nok"},
		{"convert currency at the exchange rate on a specific date", "/exchange/{value}/{base}/{counter}/{date}", "/exchange/1200.5/sek/nok/2019-01-01"},
		{"check if the current date is a bank day", "/bankday", "/bankday"},
		{"check if a specific date is a bank day", "/bankday/{date}", "/bankday/2019-01-01"},
	}
)

func main() {
	rb := riksbank.New(riksbank.Config{})
	r := mux.NewRouter()

	var production bool
	flag.BoolVar(&production, "production", false, "use this flag to run the application in production mode")
	flag.Parse()

	r.HandleFunc(`/`, homeHandler())
	r.HandleFunc(`/exchange/rate/{base:[a-zA-Z]{3}}/{counter:[a-zA-Z]{3}}/{date}`, exchangeRateHandler(rb))
	r.HandleFunc(`/exchange/rate/{base:[a-zA-Z]{3}}/{counter:[a-zA-Z]{3}}`, exchangeRateHandler(rb))
	r.HandleFunc(`/exchange/{value:\d+.\d+}/{base:[a-zA-Z]{3}}/{counter:[a-zA-Z]{3}}/{date}`, exchangeHandler(rb))
	r.HandleFunc(`/exchange/{value:\d+}/{base:[a-zA-Z]{3}}/{counter:[a-zA-Z]{3}}/{date}`, exchangeHandler(rb))
	r.HandleFunc(`/exchange/{value:\d+.\d+}/{base:[a-zA-Z]{3}}/{counter:[a-zA-Z]{3}}`, exchangeHandler(rb))
	r.HandleFunc(`/exchange/{value:\d+}/{base:[a-zA-Z]{3}}/{counter:[a-zA-Z]{3}}`, exchangeHandler(rb))
	r.HandleFunc(`/bankday/{date}`, dayHandler(rb))
	r.HandleFunc(`/bankday`, dayHandler(rb))

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
