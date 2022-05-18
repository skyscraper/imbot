package model

import (
	"fmt"
	"github.com/alpacahq/alpaca-trade-api-go/v2/marketdata"
	"time"
)

type SymbolInfo struct {
	Target   uint32
	Mult     float64
	InFlight bool
	DesPos   int
	OrderId  string
}

func getAvgVolAndClose(client marketdata.Client, symbol string, start, end time.Time) (averageVolume float64, close float64, err error) {
	var totalVolume uint64
	var n int
	for item := range client.GetBarsAsync(symbol, marketdata.GetBarsParams{
		Start: start,
		End:   end,
		Feed:  "iex",
	}) {
		if err = item.Error; err != nil {
			return
		}
		totalVolume += item.Bar.Volume
		close = item.Bar.Close
		n++
	}
	if n == 0 {
		return
	}
	averageVolume = float64(totalVolume) / float64(n)
	return
}

func GetSymbolTargets(symbols []string, maxPositionNotional float64, apiKey, apiSecret string) map[string]SymbolInfo {
	symbolTargets := make(map[string]SymbolInfo)
	// set time and dates
	nyc, err := time.LoadLocation("America/New_York")
	if err != nil {
		fmt.Println(err)
		return symbolTargets
	}
	now := time.Now().In(nyc)
	year, month, day := now.Date()
	start := time.Date(year, month, day-7, 0, 0, 0, 0, nyc)
	end := time.Date(year, month, day, 0, 0, 0, 0, nyc)
	client := marketdata.NewClient(marketdata.ClientOpts{
		ApiKey:    apiKey,
		ApiSecret: apiSecret,
	})
	for _, symbol := range symbols {
		averageVolume, closePrice, err := getAvgVolAndClose(client, symbol, start, end)
		if err != nil {
			fmt.Println(err)
			return symbolTargets
		}
		target := uint32(averageVolume / barsPerDay)
		mult := maxPositionNotional / closePrice
		symbolTargets[symbol] = SymbolInfo{Target: target, Mult: mult}
	}
	return symbolTargets
}
