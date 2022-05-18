package main

import (
	"context"
	"fmt"
	"github.com/alpacahq/alpaca-trade-api-go/v2/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v2/marketdata/stream"
	"github.com/shopspring/decimal"
	"log"
	"os"
	"os/signal"
	"skyscraper/imbot/model"
	"skyscraper/imbot/websocket"
	"syscall"
	"time"
)

func createTradeChannelsAndHandlers(tradeClient alpaca.Client, symbols []string, maxSymbolNotional float64) (map[string]chan stream.Trade, map[string]chan alpaca.TradeUpdate) {
	var tradeChannels = make(map[string]chan stream.Trade)
	var tradeUpdateChannels = make(map[string]chan alpaca.TradeUpdate)
	symbolTargets := model.GetSymbolTargets(symbols, maxSymbolNotional, websocket.ApiKey, websocket.ApiSecret)
	for symbol, symbolInfo := range symbolTargets {
		marketDataChannel := make(chan stream.Trade, 1000)
		tradeUpdateChannel := make(chan alpaca.TradeUpdate, 1000)
		tradeChannels[symbol] = marketDataChannel
		tradeUpdateChannels[symbol] = tradeUpdateChannel
		account, err := tradeClient.GetAccount()
		if err != nil {
			fmt.Printf("get account: %s", err)
		}
		// spawn listeners
		go func(symbol string, symbolInfo model.SymbolInfo) {
			bar := model.Bar{}
			bar.Variance = 0.000001
			var entry decimal.Decimal
			var wins, bets int
			for {
				select {
				case trade := <-marketDataChannel:
					bar.Update(trade)
					if bar.Vol >= symbolInfo.Target {
						orderInfo := bar.ResetAndGetOrderQty(&symbolInfo)
						if orderInfo.Qty > 0 {
							order, err := model.SubmitOrder(tradeClient, account.ID, symbol, orderInfo, &symbolInfo)
							if err != nil {
								return
							}
							(&symbolInfo).OrderId = order.ID
						} else if symbolInfo.InFlight && bar.Count >= 3 {
							fmt.Printf("%s position is stale, canceling bracket + closing...\n", symbol)
							err := tradeClient.CancelOrder(symbolInfo.OrderId)
							if err != nil {
								fmt.Printf("error occurred while trying to cancel order for %s: %s\n", symbol, err)
							}
							err = tradeClient.ClosePosition(symbol)
							if err != nil {
								fmt.Printf("error occurred while trying to close position for %s: %s\n", symbol, err)
							}
						}
					}
				case tradeUpdate := <-tradeUpdateChannel:
					switch tradeUpdate.Event {
					case "fill":
						positionSize := int(tradeUpdate.PositionQty.IntPart())
						if positionSize == 0 {
							profit := tradeUpdate.Order.FilledAvgPrice.Sub(entry).Div(entry).Mul(decimal.NewFromInt(10000))
							if tradeUpdate.Order.Side == alpaca.Buy {
								profit = profit.Mul(decimal.NewFromInt(-1))
							}
							(&symbolInfo).InFlight = false
							fmt.Printf("%s position closed, profit %s bps \n", symbol, profit)
							bets += 1
							if profit.GreaterThan(decimal.Zero) {
								wins += 1
							}
							fmt.Printf("%s overall win rate: %d / %d\n", symbol, wins, bets)
						} else if positionSize == symbolInfo.DesPos {
							entry = *tradeUpdate.Order.FilledAvgPrice
						} else {
							fmt.Printf("received %s fill with position size %d and des pos %d", symbol, positionSize, symbolInfo.DesPos)
						}
					case "partial_fill":
						continue
					case "new":
						continue
					case "canceled":
						continue
					case "replaced":
						continue
					default:
						fmt.Println("received non-fill: ", tradeUpdate.Event)
						fmt.Println(tradeUpdate)
					}
				}
			}
		}(symbol, symbolInfo)
	}
	return tradeChannels, tradeUpdateChannels
}

func clearOrdersAndPositions(tradeClient alpaca.Client) {
	err := tradeClient.CancelAllOrders()
	if err != nil {
		fmt.Println(err)
		return
	}
	err = tradeClient.CloseAllPositions()
	if err != nil {
		fmt.Println(err)
		return
	}
}

func getMaxSymbolNotional(symbolCount int64, tradeClient alpaca.Client) float64 {
	acc, err := tradeClient.GetAccount()
	if err != nil {
		fmt.Println(err)
		return 0
	}
	fmt.Printf("buying power: %s\n", acc.BuyingPower)
	// only use 90% of max buying power
	maxSymbolNotional := acc.BuyingPower.Div(decimal.NewFromInt(symbolCount)).Mul(decimal.NewFromFloat(0.9))
	fmt.Printf("max notional per symbol: %s\n", maxSymbolNotional)
	result, _ := maxSymbolNotional.Float64()
	return result
}

func filterSymbols(symbols []string, tradeClient alpaca.Client) []string {
	var result []string
	for _, symbol := range symbols {
		asset, _ := tradeClient.GetAsset(symbol)
		if asset.Tradable && asset.Marginable && asset.Shortable && asset.EasyToBorrow {
			result = append(result, symbol)
		} else {
			fmt.Printf("Excluding %s, fails tradability checks\n", symbol)
		}
	}
	return result
}

func main() {
	// context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// setting up cancelling upon interrupt
	quitChannel := make(chan os.Signal, 1)
	signal.Notify(quitChannel, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// trade client
	tradeClient := alpaca.NewClient(alpaca.ClientOpts{
		ApiKey:    websocket.ApiKey,
		ApiSecret: websocket.ApiSecret,
		BaseURL:   "https://paper-api.alpaca.markets",
	})

	// clear any positions
	clearOrdersAndPositions(tradeClient)

	// declare and filter symbols
	initialSymbols := []string{"DOMO", "SQ", "MRO", "AAPL", "GM", "SNAP", "SHOP", "SPLK", "BA", "AMZN", "SUI", "SUN", "TSLA",
		"CGC", "SPWR", "NIO", "CAT", "MSFT", "PANW", "OKTA", "TWTR", "TM", "GE", "ATVI", "GS", "BAC", "MS", "TWLO",
		"QCOM", "IBM"}
	symbols := filterSymbols(initialSymbols, tradeClient)

	// get max notional per symbol
	maxSymbolNotional := getMaxSymbolNotional(int64(len(symbols)), tradeClient)

	// set up channels
	var tradeChannels, tradeUpdateChannels = createTradeChannelsAndHandlers(tradeClient, symbols, maxSymbolNotional)

	// connect and subscribe to trade updates - this will then push the updates to the tradeUpdateChannels
	err := websocket.AlpacaPrivateStream(tradeUpdateChannels)
	if err != nil {
		return
	}

	// market data handler
	tradeHandler := func(trade stream.Trade) {
		ch := tradeChannels[trade.Symbol]
		ch <- trade
	}

	// set up streaming client
	c := stream.NewStocksClient(
		"iex",
		stream.WithTrades(tradeHandler, symbols...),
		stream.WithCredentials(websocket.ApiKey, websocket.ApiSecret),
		stream.WithProcessors(1),
	)

	if err := c.Connect(ctx); err != nil {
		log.Fatalf("could not establish connection, error: %s", err)
	}
	fmt.Println("established connection")

	// starting a goroutine that checks whether the client has terminated
	go func() {
		err := <-c.Terminated()
		if err != nil {
			log.Fatalf("terminated with error: %s", err)
		}
		fmt.Println("exiting")
		os.Exit(0)
	}()

	fmt.Println("setup done")

	// set up end of day listener
	go func() {
		for {
			clock, _ := tradeClient.GetClock()
			if clock.NextClose.Sub(clock.Timestamp) < 7*time.Minute {
				quitChannel <- syscall.Signal(1)
			}
			time.Sleep(1 * time.Minute)
		}
	}()

	// wait for shutdown signal...
	<-quitChannel
	fmt.Println("received signal, liquidating and shutting down")
	// clear orders and positions
	clearOrdersAndPositions(tradeClient)
	time.Sleep(5 * time.Second)
	// cancel
	cancel()
}
