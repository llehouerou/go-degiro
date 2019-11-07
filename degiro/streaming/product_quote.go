package streaming

import "github.com/shopspring/decimal"

type ProductQuote struct {
	IssueId   string
	FullName  string
	LastPrice decimal.Decimal
	BidPrice  decimal.Decimal
	AskPrice  decimal.Decimal
	OpenPrice decimal.Decimal
	LowPrice  decimal.Decimal
	HighPrice decimal.Decimal
	BidVolume decimal.Decimal
	AskVolume decimal.Decimal
}
