# go-degiro

go-degiro is a Go client library for the unofficial Degiro API & Vwd streaming API

## Install

go get github.com/llehouerou/go-degiro

## Usage

### Degiro API

```go
    client := degiro.NewClient(http.DefaultClient)
    
    client.UpdatePeriod = 2 * time.Second
    client.HistoricalPositionUpdatePeriod = 1 * time.Minute
    client.StreamingUpdatePeriod = 1 * time.Second

    err := client.Login("username", "password")
    if err != nil {
        panic(err)
    }
    time.Sleep(2 * time.Second) // let time for first update

    // Balance
    balance := client.GetBalance()
    fmt.Printf("%+v\n", balance)

    // Get streaming quote for product
    if product, ok, err := client.SearchProduct("APPLE"); err == nil && ok {
        err = client.SubscribeQuotes([]string{product.VwdId})
        time.Sleep(2 * time.Second) // let time to update quote
        quote := client.GetQuote(product.VwdId)
        fmt.Printf("%+v\n", quote)
    }
```

## License

[MIT License](LICENSE)