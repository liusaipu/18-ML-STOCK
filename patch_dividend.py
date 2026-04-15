with open('downloader/eastmoney.go', 'r', encoding='utf-8') as f:
    lines = f.readlines()
for i, line in enumerate(lines):
    if 'quote.PB = parseAnyFloat(resp.Data["f23"])' in line:
        lines.insert(i+1, '\t\t\t\tquote.DividendYield = parseAnyFloat(resp.Data["f133"]) / 100 // 接口返回百分比数值，转为小数\n')
        break
with open('downloader/eastmoney.go', 'w', encoding='utf-8') as f:
    f.writelines(lines)
print('done')
