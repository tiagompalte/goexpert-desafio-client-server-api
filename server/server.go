package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type AwesomeApiResponse struct {
	Usdbrl struct {
		Code       string `json:"code"`
		Codein     string `json:"codein"`
		Name       string `json:"name"`
		High       string `json:"high"`
		Low        string `json:"low"`
		VarBid     string `json:"varBid"`
		PctChange  string `json:"pctChange"`
		Bid        string `json:"bid"`
		Ask        string `json:"ask"`
		Timestamp  string `json:"timestamp"`
		CreateDate string `json:"create_date"`
	} `json:"USDBRL"`
}

type Response struct {
	Value float64 `json:"value"`
}

func openConnection() (*sql.DB, error) {
	return sql.Open("sqlite3", "exchange.db")
}

func createTable(db *sql.DB) error {
	sts := `CREATE TABLE IF NOT EXISTS exchange(
		id INTEGER PRIMARY KEY, 
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, 
		value NUMERIC(8,4) NOT NULL);`
	_, err := db.Exec(sts)
	if err != nil {
		return err
	}
	return nil
}

func request(ctx context.Context) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://economia.awesomeapi.com.br/json/last/USD-BRL", nil)
	if err != nil {
		return 0, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var bodyResponse AwesomeApiResponse
	err = json.Unmarshal(body, &bodyResponse)
	if err != nil {
		return 0, err
	}

	value, err := strconv.ParseFloat(bodyResponse.Usdbrl.Bid, 64)
	if err != nil {
		return 0, err
	}

	return value, nil
}

func save(ctx context.Context, db *sql.DB, value float64) error {
	_, err := db.ExecContext(ctx, "INSERT INTO exchange(value) VALUES(?);", value)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	db, err := openConnection()
	if err != nil {
		panic(err)
	}
	defer db.Close()

	err = createTable(db)
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/cotacao", func(w http.ResponseWriter, r *http.Request) {
		ctxReq, cancelReq := context.WithTimeout(r.Context(), 200*time.Millisecond)
		defer cancelReq()

		value, err := request(ctxReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ctxSave, cancelSave := context.WithTimeout(r.Context(), 10*time.Millisecond)
		defer cancelSave()

		err = save(ctxSave, db, value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp := Response{
			Value: value,
		}

		err = json.NewEncoder(w).Encode(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.ListenAndServe(":8080", nil)
}
