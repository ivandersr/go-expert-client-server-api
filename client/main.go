package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type Response struct {
	Bid string `json:"bid"`
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*300)
	defer cancel()
	requestForQuotation(ctx)
}

func requestForQuotation(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:8080/cotacao", nil)
	if err != nil {
		log.Printf("Error mounting the request with context: %s", err)
		return
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error making the request: %s", err)
		return
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusRequestTimeout {
		log.Println("Server timeout")
		return
	}
	outFile, err := os.OpenFile("cotacao.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Error creating/opening \"cotacao.txt\": %s", err)
		return
	}
	defer outFile.Close()
	contents, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("Error reading response body: %s", err)
		return
	}

	var response Response
	err = json.Unmarshal(contents, &response)
	if err != nil {
		log.Printf("Error parsing response: %s", err)
		return
	}
	_, err = outFile.WriteString("Dólar: " + response.Bid + "\n")
	if err != nil {
		log.Printf("Error writing to file: %s", err)
		return
	}

	select {
	case <-req.Context().Done():
		log.Println("timeout")
	default:
		log.Println("Quotation persisted successfully")
	}
}
