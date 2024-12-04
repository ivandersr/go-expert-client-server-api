package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type APIResponse struct {
	USDBRL struct {
		Code       string `json:"code"`
		Name       string `json:"name"`
		Bid        string `json:"bid"`
		CreateDate string `json:"create_date"`
	} `json:"USDBRL"`
}

type Response struct {
	Bid string `json:"bid"`
}

func main() {
	http.HandleFunc("/cotacao", handler)
	http.ListenAndServe(":8080", nil)
}

func handler(w http.ResponseWriter, r *http.Request) {
	ctxReq, cancelReq := context.WithTimeout(context.Background(), time.Millisecond*200)

	// 10ms para persistir algo no banco parece ser muito baixo, não obtive
	// sucesso em nenhum insert com esse timeout, então escolhi 100 ms deliberadamente
	// visto que o a soma de ambos timeouts ainda estão dentro do timeout esperado pelo client
	ctxDb, cancelDb := context.WithTimeout(context.Background(), time.Millisecond*100)

	db, err := sql.Open("sqlite3", "dolar.db")
	if err != nil {
		fmt.Printf("error connecting to sqlite database: %s\n", err)
		os.Exit(1)
	}

	defer cancelReq()
	defer cancelDb()
	defer db.Close()

	resp, err := dolarQuotation(ctxReq, ctxDb, db)
	if err != nil {
		log.Printf("Request error: %s", err)
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		log.Printf("Error parsing http response: %s", err)
		return
	}
}

func dolarQuotation(ctxReq, ctxDb context.Context, db *sql.DB) (*Response, error) {
	requestURL := "https://economia.awesomeapi.com.br/json/last/USD-BRL"
	req, err := http.NewRequestWithContext(ctxReq, "GET", requestURL, nil)
	if err != nil {
		log.Printf("error setting context to http request: %s\n", err)
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("error making request to external API: %s\n", err)
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("error reading response body: %s", err)
		return nil, err
	}
	var apiResponse APIResponse
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		log.Printf("error parsing response body: %s", err)
		return nil, err
	}

	_, err = db.ExecContext(
		ctxDb,
		"INSERT INTO dolar_quotations (code, name, bid, create_date) values (?, ?, ?, ?);",
		apiResponse.USDBRL.Code,
		apiResponse.USDBRL.Name,
		apiResponse.USDBRL.Bid,
		apiResponse.USDBRL.CreateDate,
	)

	if err != nil {
		log.Printf("error inserting quotation record: %s", err)
	}

	resp := Response{apiResponse.USDBRL.Bid}

	select {
	case <-ctxReq.Done():
		log.Println("Process exited. Request timeout reached")
		return nil, errors.New("request timeout")
	case <-ctxDb.Done():
		log.Println("Process exited. Database timeout reached")
		return nil, errors.New("database timeout")
	default:
		log.Println("Quotation persisted successfully")
		return &resp, nil
	}
}
