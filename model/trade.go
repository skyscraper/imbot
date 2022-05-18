package model

import (
	"fmt"
	"github.com/alpacahq/alpaca-trade-api-go/v2/alpaca"
	"github.com/shopspring/decimal"
)

func fromFloat(price float64) decimal.Decimal {
	return decimal.NewFromFloat(float64(int(price*100)) / 100)
}

func SubmitOrder(tradeClient alpaca.Client, accountId, symbol string, orderInfo OrderInfo, symbolInfo *SymbolInfo) (*alpaca.Order, error) {
	if orderInfo.Qty > 0 {
		decimalQty := decimal.NewFromInt(int64(orderInfo.Qty))
		tp := fromFloat(orderInfo.TakeProfit)
		sp := fromFloat(orderInfo.StopLoss)
		order, err := tradeClient.PlaceOrder(alpaca.PlaceOrderRequest{
			AccountID:   accountId,
			AssetKey:    &symbol,
			Qty:         &decimalQty,
			Side:        orderInfo.Side,
			Type:        alpaca.Market,
			TimeInForce: alpaca.Day,
			OrderClass:  alpaca.Bracket,
			TakeProfit:  &alpaca.TakeProfit{LimitPrice: &tp},
			StopLoss:    &alpaca.StopLoss{StopPrice: &sp},
		})
		if err == nil {
			fmt.Printf("Market order of | %d %s %s | completed\n", orderInfo.Qty, symbol, orderInfo.Side)
		} else {
			fmt.Printf("Order of | %d %s %s | did not go through: %s\n", orderInfo.Qty, symbol, orderInfo.Side, err)
			symbolInfo.InFlight = false
		}
		return order, err
	}
	fmt.Printf("Quantity is <= 0, order of | %d %s %s | not sent\n", orderInfo.Qty, symbol, orderInfo.Side)
	return nil, nil
}
