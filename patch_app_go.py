with open('app.go', 'r', encoding='utf-8') as f:
    content = f.read()

marker = '// GetStockQuote 获取股票实时行情（带15分钟本地缓存）'
end_marker = '\treturn quote, nil\n}'

start = content.find(marker)
end = content.find(end_marker, start) + len(end_marker)

if start == -1 or end == -1:
    print('not found')
    exit(1)

old = content[start:end]

new = '''// GetStockQuote 获取股票实时行情（带15分钟本地缓存）
func (a *App) GetStockQuote(symbol string) (*downloader.StockQuote, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}

	// 尝试读取缓存（15分钟），同时校验数据合理性
	cached, err := a.storage.LoadStockQuote(symbol)
	if err == nil && cached != nil {
		path := filepath.Join(a.storage.DataDir(), "data", symbol, "quote.json")
		info, err := os.Stat(path)
		if err == nil && time.Since(info.ModTime()) < 15*time.Minute {
			// 校验缓存数据是否合理（过滤掉错误解析的巨大盘百分比或时间戳）
			if cached.CurrentPrice > 0 && cached.ChangePercent > -50 && cached.ChangePercent < 50 {
				a.fillShareholderReturnRate(symbol, cached)
				return cached, nil
			}
		}
	}

	// 拆分 symbol
	parts := strings.Split(symbol, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的股票代码格式: %s", symbol)
	}
	code := parts[0]
	market := strings.ToUpper(parts[1])

	// 从网络获取
	quote, err := downloader.FetchStockQuote(market, code)
	if err != nil {
		return nil, fmt.Errorf("获取行情失败: %w", err)
	}
	_ = a.storage.SaveStockQuote(symbol, quote)
	a.fillShareholderReturnRate(symbol, quote)
	return quote, nil
}

// fillShareholderReturnRate 根据最新 Profile 数据填充股东回报率
func (a *App) fillShareholderReturnRate(symbol string, quote *downloader.StockQuote) {
	if quote == nil {
		return
	}
	profile, _ := a.GetStockProfile(symbol)
	if profile != nil && profile.ROE > 0 && profile.PB > 0 {
		quote.ShareholderReturnRate = profile.ROE / profile.PB
		if quote.DividendYield > 0 {
			quote.ShareholderReturnRate += quote.DividendYield
		}
	}
}'''

content = content[:start] + new + content[end:]
with open('app.go', 'w', encoding='utf-8') as f:
    f.write(content)
print('done')
