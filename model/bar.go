package model

import (
	"fmt"
	"github.com/alpacahq/alpaca-trade-api-go/v2/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v2/marketdata/stream"
	"math"
)

const (
	barMinutes = 5
	barsPerDay = 6.5 * (60 / barMinutes)
	volAlpha   = 2.0 / (1.0 + 12.0)
)

type Bar struct {
	Close         float64
	buyVol        uint32
	sellVol       uint32
	Vol           uint32
	Up            bool
	Variance      float64
	prevImbalance float64
	prevClose     float64
	Count         uint8
}

func (bar *Bar) Update(trade stream.Trade) {
	delta := trade.Price - bar.Close
	if delta > 0.001 {
		bar.Up = true
		bar.buyVol += trade.Size
	} else if delta < -0.001 {
		bar.Up = false
		bar.sellVol += trade.Size
	} else {
		if bar.Up {
			bar.buyVol += trade.Size
		} else {
			bar.sellVol += trade.Size
		}
	}
	bar.Vol += trade.Size
	bar.Close = trade.Price
}

func (bar *Bar) reset() {
	bar.buyVol = 0
	bar.sellVol = 0
	bar.Vol = 0
	var rtn float64
	if bar.prevClose == 0 {
		rtn = 0
	} else {
		rtn = pctReturn(bar.prevClose, bar.Close)
	}
	bar.Variance = ewmStep(bar.Variance, math.Pow(rtn, 2), volAlpha)
	bar.prevClose = bar.Close
	bar.Count += 1
}

func (bar Bar) GetImbalance() float64 {
	return (float64(bar.buyVol) - float64(bar.sellVol)) / float64(bar.Vol)
}

type OrderInfo struct {
	Qty        int
	Side       alpaca.Side
	TakeProfit float64
	StopLoss   float64
}

var emptyOrder = OrderInfo{Qty: 0}

func (bar *Bar) ResetAndGetOrderQty(info *SymbolInfo) OrderInfo {
	imbalance := bar.GetImbalance()
	delta := int(imbalance * info.Mult)
	bar.reset()
	if math.Abs(imbalance) < 0.333 || delta == 0 || info.InFlight {
		return emptyOrder
	}
	annualVol := math.Sqrt(bar.Variance * barsPerDay * 252)
	pct := 0.015 * annualVol
	if pct < 0.002 {
		return emptyOrder
	}
	gain := imbalance * pct
	loss := gain * 0.85
	takeProfit := bar.Close * (1 + gain)
	stopLoss := bar.Close * (1 - loss)
	if math.Abs(takeProfit-bar.Close) < 0.01 || math.Abs(stopLoss-bar.Close) < 0.01 {
		return emptyOrder
	}
	fmt.Printf("annualized volatility: %f%%\n", 100*annualVol)
	info.InFlight = true
	bar.Count = 0
	info.DesPos = delta
	var side alpaca.Side
	var qty int
	if delta > 0 {
		side = alpaca.Buy
		qty = delta
	} else {
		side = alpaca.Sell
		qty = -1 * delta
	}
	return OrderInfo{Qty: qty, Side: side, TakeProfit: takeProfit, StopLoss: stopLoss}
}
