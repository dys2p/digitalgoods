package main

import (
	"encoding/json"
	"net/http"
)

type HealthItem struct {
	CryptoCode string
	Synced     bool
}

// provides the same simple API as https://github.com/dys2p/xmrhealthd/
func health(w http.ResponseWriter, r *http.Request) {

	status, err := btcpayStore.GetServerStatus()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(nil)
		return
	}

	result := []HealthItem{}
	for _, syncStatus := range status.SyncStatuses {
		result = append(result, HealthItem{
			CryptoCode: syncStatus.CryptoCode,
			Synced:     syncStatus.ChainHeight == syncStatus.SyncHeight,
		})
	}

	responseData, _ := json.Marshal(result)
	w.Header().Add("Content-Type", "application/json")
	w.Write(responseData)
}
