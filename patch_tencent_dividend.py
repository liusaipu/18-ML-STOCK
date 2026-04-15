with open('downloader/eastmoney.go', 'r', encoding='utf-8') as f:
    content = f.read()
old = '''\t\tif !isHK && len(parts) > 49 {
\t\t\tquote.VolumeRatio = parseStrFloat(parts[49])
\t\t}
\t\t// 时间格式'''
new = '''\t\tif !isHK && len(parts) > 49 {
\t\t\tquote.VolumeRatio = parseStrFloat(parts[49])
\t\t}
\t\t// 股息率：腾讯接口 parts[62] 为股息率（百分比数值）
\t\tif len(parts) > 62 {
\t\t\tquote.DividendYield = parseStrFloat(parts[62]) / 100
\t\t}
\t\t// 时间格式'''
if old in content:
    content = content.replace(old, new)
    with open('downloader/eastmoney.go', 'w', encoding='utf-8') as f:
        f.write(content)
    print('done')
else:
    print('not found')
