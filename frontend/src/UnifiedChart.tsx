import { useEffect, useMemo, useRef, useState } from 'react'
import * as echarts from 'echarts'
import type { downloader } from '../wailsjs/go/models'
import { GetStockKlines, GetStockQuote } from '../wailsjs/go/main/App'

type KlineData = downloader.KlineData
type StockQuote = downloader.StockQuote

interface Props {
  code: string
  quote?: StockQuote
}

const colors = {
  up: '#ef4444',
  down: '#22c55e',
  ma5: '#fbbf24',
  ma10: '#60a5fa',
  ma30: '#a78bfa',
  ma60: '#f87171',
  macd: '#f59e0b',
  signal: '#3b82f6',
  histPositive: '#ef4444',
  histNegative: '#22c55e',
  rsi6: '#f97316',
  rsi12: '#a78bfa',
  rsi24: '#94a3b8',
  bbUpper: '#ef4444',
  bbMid: '#f59e0b',
  bbLower: '#10b981',
}

function calcEMA(arr: number[], period: number): (number | null)[] {
  const k = 2 / (period + 1)
  const ema: (number | null)[] = []
  for (let i = 0; i < arr.length; i++) {
    if (i === 0) ema.push(arr[0])
    else ema.push(arr[i] * k + (ema[i - 1] as number) * (1 - k))
  }
  return ema
}

function calcMA(arr: number[], period: number): (number | null)[] {
  const ma: (number | null)[] = []
  for (let i = 0; i < arr.length; i++) {
    if (i < period - 1) { ma.push(null); continue }
    let sum = 0
    for (let j = i - period + 1; j <= i; j++) sum += arr[j]
    ma.push(sum / period)
  }
  return ma
}

function padArray<T>(arr: T[], size: number): (T | '-')[] {
  const padCount = size - arr.length
  if (padCount <= 0) return arr as (T | '-')[]
  return [...Array(padCount).fill('-'), ...arr]
}

function calculateIndicators(data: KlineData[]) {
  const closes = data.map(d => d.close)
  const ma5 = calcMA(closes, 5)
  const ma10 = calcMA(closes, 10)
  const ma30 = calcMA(closes, 30)
  const ma60 = calcMA(closes, 60)

  const ema12 = calcEMA(closes, 12)
  const ema26 = calcEMA(closes, 26)
  const dif: (number | null)[] = ema12.map((v, i) => (v == null || ema26[i] == null) ? null : v - ema26[i]!)
  const validDif = dif.filter((v): v is number => v != null)
  const validDea = calcEMA(validDif, 9)
  const dea: (number | null)[] = []
  let deaIdx = 0
  for (let i = 0; i < dif.length; i++) {
    if (dif[i] == null) dea.push(null)
    else dea.push(validDea[deaIdx++] ?? null)
  }
  const hist: (number | null)[] = dif.map((v, i) => (v == null || dea[i] == null) ? null : 2 * (v - dea[i]!))

  function calcRSI(period: number): (number | null)[] {
    const result: (number | null)[] = []
    let avgGain = 0
    let avgLoss = 0
    for (let i = 0; i < closes.length; i++) {
      if (i === 0) { result.push(null); continue }
      const diff = closes[i] - closes[i - 1]
      const gain = diff > 0 ? diff : 0
      const loss = diff < 0 ? -diff : 0
      if (i < period) {
        // 积累初始 period 个变化值
        avgGain += gain
        avgLoss += loss
        result.push(null)
      } else if (i === period) {
        // 第一个 RSI：简单平均
        avgGain += gain
        avgLoss += loss
        avgGain /= period
        avgLoss /= period
        result.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss))
      } else {
        // 后续 RSI：Wilder's smoothing
        avgGain = (avgGain * (period - 1) + gain) / period
        avgLoss = (avgLoss * (period - 1) + loss) / period
        result.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss))
      }
    }
    return result
  }
  const rsi6 = calcRSI(6)
  const rsi12 = calcRSI(12)
  const rsi24 = calcRSI(24)

  const bbUpper: (number | null)[] = [], bbMid: (number | null)[] = [], bbLower: (number | null)[] = []
  for (let i = 0; i < closes.length; i++) {
    if (i < 19) { bbUpper.push(null); bbMid.push(null); bbLower.push(null); continue }
    const slice = closes.slice(i - 19, i + 1)
    const mean = slice.reduce((a, b) => a + b, 0) / 20
    const std = Math.sqrt(slice.reduce((sq, n) => sq + Math.pow(n - mean, 2), 0) / 20)
    bbMid.push(mean)
    bbUpper.push(mean + 2 * std)
    bbLower.push(mean - 2 * std)
  }

  return { dif, dea, hist, rsi6, rsi12, rsi24, bbUpper, bbMid, bbLower, ma5, ma10, ma30, ma60 }
}

function fmt2(v: any): string {
  if (v == null) return '-'
  const n = Number(v)
  if (isNaN(n)) return '-'
  return n.toFixed(2)
}
function fmt3(v: any): string {
  if (v == null) return '-'
  const n = Number(v)
  if (isNaN(n)) return '-'
  return n.toFixed(3)
}
function fmt1(v: any): string {
  if (v == null) return '-'
  const n = Number(v)
  if (isNaN(n)) return '-'
  return n.toFixed(1)
}

export function UnifiedChart({ code, quote: propQuote }: Props) {
  const chartRef = useRef<HTMLDivElement>(null)
  const chartInstanceRef = useRef<echarts.ECharts | null>(null)
  const [rawData, setRawData] = useState<KlineData[]>([])
  const [localQuote, setLocalQuote] = useState<StockQuote | undefined>(propQuote)
  const [loading, setLoading] = useState(false)
  const [isExpanded, setIsExpanded] = useState(false)

  // 如果 propQuote 变化，同步更新 localQuote
  useEffect(() => {
    setLocalQuote(propQuote)
  }, [propQuote])

  // K 线数据加载
  useEffect(() => {
    if (!code) return
    setLoading(true)
    GetStockKlines(code)
      .then((list) => setRawData(list || []))
      .catch(() => setRawData([]))
      .finally(() => setLoading(false))
  }, [code])

  // 自己获取行情（如果 propQuote 为 null/undefined）
  useEffect(() => {
    if (!code || propQuote) return
    GetStockQuote(code)
      .then((q) => {
        if (q && q.currentPrice > 0) {
          setLocalQuote(q)
        }
      })
      .catch(() => {})
  }, [code, propQuote])

  const data = useMemo(() => {
    if (rawData.length === 0) return []
    const quote = localQuote
    const hasTurnover = rawData.some(d => d.turnoverRate > 0)
    if (hasTurnover || !quote || quote.circulatingMarketCap <= 0 || quote.currentPrice <= 0) {
      return rawData
    }
    const circulatingShares = quote.circulatingMarketCap / quote.currentPrice
    return rawData.map(d => ({
      ...d,
      turnoverRate: (d.volume * 100 / circulatingShares) * 100,
    }))
  }, [rawData, localQuote])

  useEffect(() => {
    if (!chartRef.current || data.length === 0) return

    if (chartInstanceRef.current) {
      chartInstanceRef.current.dispose()
    }

    const chart = echarts.init(chartRef.current, 'dark', { renderer: 'canvas' })
    chartInstanceRef.current = chart

    chart.getZr().on('dblclick', () => setIsExpanded(v => !v))

    const displaySize = isExpanded ? 250 : 120
    const { dif, dea, hist, rsi6, rsi12, rsi24, bbUpper, bbMid, bbLower, ma5, ma10, ma30, ma60 } = calculateIndicators(data)

    const displayData = data.slice(-displaySize)
    const padCount = displaySize - displayData.length
    const dates = [...Array(padCount).fill(''), ...displayData.map(d => d.time)]

    const safePad = Array(padCount).fill('-')
    const candleData = [...safePad, ...displayData.map(d => [d.open, d.close, d.low, d.high])]
    const turnoverData = [
      ...safePad,
      ...displayData.map((d: KlineData) => ({
        value: d.turnoverRate,
        itemStyle: { color: d.close >= d.open ? 'rgba(239,68,68,0.35)' : 'rgba(34,197,94,0.35)' },
      })),
    ]

    const sliceDisplay = (arr: (number | null)[]) => {
      const sliced = arr.slice(-displaySize)
      return padArray(sliced, displaySize)
    }

    const xAxisLabelInterval = isExpanded ? 39 : 19

    const option: echarts.EChartsOption = {
      backgroundColor: 'transparent',
      animation: false,
      legend: {
        data: ['K线', 'MA5', 'MA10', 'MA30', 'MA60'],
        top: 8,
        right: 10,
        textStyle: { color: '#94a3b8', fontSize: 11 },
        itemStyle: { borderWidth: 0 },
        itemGap: 8,
      },
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'cross',
          link: [{ xAxisIndex: 'all' }] as any,
          label: { show: false },
        },
        backgroundColor: 'rgba(15, 23, 42, 0.95)',
        borderColor: 'rgba(148, 163, 184, 0.25)',
        borderWidth: 1,
        textStyle: { color: '#e2e8f0', fontSize: 12 },
        padding: 0,
        formatter: (params: any) => {
          if (!params || params.length === 0) return ''
          const date = params[0].axisValue || ''
          if (!date) return ''

          const leftItems: string[] = []
          const candle = params.find((p: any) => p.seriesName === 'K线')
          if (candle) {
            const idx = candle.dataIndex - padCount
            const d = displayData[idx]
            if (d) {
              const o = d.open, c = d.close, l = d.low, h = d.high
              const prevClose = idx > 0 ? displayData[idx - 1].close : o
              const change = c - prevClose
              const changePct = prevClose !== 0 ? (change / prevClose) * 100 : 0
              const changeColor = change >= 0 ? '#ef4444' : '#22c55e'
              const changeSign = change >= 0 ? '+' : ''
              leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">开盘</span><span>${fmt2(o)}</span></div>`)
              leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">收盘</span><span>${fmt2(c)}</span></div>`)
              leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">涨跌额</span><span style="color:${changeColor}">${changeSign}${fmt2(change)}</span></div>`)
              leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">涨跌幅</span><span style="color:${changeColor}">${changeSign}${fmt2(changePct)}%</span></div>`)
              leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">最低</span><span>${fmt2(l)}</span></div>`)
              leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">最高</span><span>${fmt2(h)}</span></div>`)
            }
          }
          params.filter((p: any) => ['MA5', 'MA10', 'MA30', 'MA60'].includes(p.seriesName)).forEach((p: any) => {
            const color = p.color || '#94a3b8'
            leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:${color}">● ${p.seriesName}</span><span>${fmt2(p.value)}</span></div>`)
          })

          const rightItems: string[] = []
          const turnover = params.find((p: any) => p.seriesName === '换手率')
          if (turnover) {
            rightItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">换手率</span><span>${turnover.value != null ? fmt2(turnover.value) + '%' : '-'}</span></div>`)
          }

          const macdParams = params.filter((p: any) => ['DIF', 'DEA', 'MACD'].includes(p.seriesName))
          if (macdParams.length) {
            if (rightItems.length) rightItems.push('<div style="border-top:1px solid rgba(148,163,184,0.12);margin:4px 0"></div>')
            macdParams.forEach((p: any) => {
              const color = p.color || '#94a3b8'
              rightItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:${color}">● ${p.seriesName}</span><span>${fmt3(p.value)}</span></div>`)
            })
          }
          const rsiParams = params.filter((p: any) => ['RSI6', 'RSI12', 'RSI24'].includes(p.seriesName))
          if (rsiParams.length) {
            if (rightItems.length) rightItems.push('<div style="border-top:1px solid rgba(148,163,184,0.12);margin:4px 0"></div>')
            rsiParams.forEach((p: any) => {
              const colorMap: Record<string, string> = { RSI6: colors.rsi6, RSI12: colors.rsi12, RSI24: colors.rsi24 }
              rightItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:${colorMap[p.seriesName] || '#94a3b8'}">● ${p.seriesName}</span><span>${fmt1(p.value)}</span></div>`)
            })
          }
          const bbParams = params.filter((p: any) => ['上轨', '中轨', '下轨'].includes(p.seriesName))
          if (bbParams.length) {
            if (rightItems.length) rightItems.push('<div style="border-top:1px solid rgba(148,163,184,0.12);margin:4px 0"></div>')
            bbParams.forEach((p: any) => {
              const color = p.color || '#94a3b8'
              rightItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:${color}">● ${p.seriesName}</span><span>${fmt2(p.value)}</span></div>`)
            })
          }

          return `
            <div style="line-height:1.65;font-size:12px">
              <div style="font-weight:600;margin-bottom:6px;color:#f0f0f0;padding:10px 14px 0">${date}</div>
              <div style="display:flex;gap:14px;padding:0 14px 10px">
                <div style="min-width:110px">${leftItems.join('')}</div>
                <div style="min-width:110px">${rightItems.join('')}</div>
              </div>
            </div>
          `
        },
      },
      axisPointer: {
        link: [{ xAxisIndex: 'all' }],
        label: { show: false },
      },
      grid: isExpanded ? [
        { left: 75, right: 16, top: 38, height: '44%' },
        { left: 75, right: 16, top: '50%', height: '11%' },
        { left: 75, right: 16, top: '62%', height: '11%' },
        { left: 75, right: 16, top: '74%', height: '11%' },
        { left: 75, right: 16, top: '86%', height: '14%' },
      ] : [
        { left: 75, right: 16, top: 38, height: 258 },
        { left: 75, right: 16, top: 304, height: 50 },
        { left: 75, right: 16, top: 362, height: 50 },
        { left: 75, right: 16, top: 420, height: 50 },
        { left: 75, right: 16, top: 478, height: 58 },
      ],
      xAxis: [
        { type: 'category', data: dates, boundaryGap: true, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { color: '#94a3b8', fontSize: 10, interval: xAxisLabelInterval }, splitLine: { show: false }, gridIndex: 0, axisPointer: { label: { show: false } } },
        { type: 'category', data: dates, boundaryGap: true, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { show: false }, splitLine: { show: false }, gridIndex: 1, axisPointer: { label: { show: false } } },
        { type: 'category', data: dates, boundaryGap: true, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { show: false }, splitLine: { show: false }, gridIndex: 2, axisPointer: { label: { show: false } } },
        { type: 'category', data: dates, boundaryGap: true, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { show: false }, splitLine: { show: false }, gridIndex: 3, axisPointer: { label: { show: false } } },
        { type: 'category', data: dates, boundaryGap: true, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { color: '#94a3b8', fontSize: 10, interval: xAxisLabelInterval }, splitLine: { show: false }, gridIndex: 4, axisPointer: { label: { show: true, backgroundColor: '#3b82f6' } } },
      ],
      yAxis: [
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, gridIndex: 0, position: 'left', axisLabel: { fontSize: 10, color: '#94a3b8', margin: 10 }, splitNumber: 5, name: 'K线', nameLocation: 'middle', nameRotate: 0, nameGap: 32, nameTextStyle: { color: '#94a3b8', fontSize: 11, align: 'right' }, axisPointer: { label: { show: true, formatter: (params: any) => fmt2(params.value) } } },
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, gridIndex: 1, position: 'left', axisLabel: { show: false }, splitNumber: 2, name: '换手', nameLocation: 'middle', nameRotate: 0, nameGap: 32, nameTextStyle: { color: '#94a3b8', fontSize: 11, align: 'right' }, axisPointer: { label: { show: true, formatter: (params: any) => fmt2(params.value) + '%' } } },
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, gridIndex: 2, position: 'left', axisLabel: { show: false }, splitNumber: 3, name: 'MACD', nameLocation: 'middle', nameRotate: 0, nameGap: 32, nameTextStyle: { color: '#94a3b8', fontSize: 11, align: 'right' }, axisPointer: { label: { show: true, formatter: (params: any) => fmt3(params.value) } } },
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, min: 0, max: 100, gridIndex: 3, position: 'left', axisLabel: { show: false }, splitNumber: 2, name: 'RSI(6,12,24)', nameLocation: 'middle', nameRotate: 0, nameGap: 32, nameTextStyle: { color: '#94a3b8', fontSize: 11, align: 'right' }, axisPointer: { label: { show: true, formatter: (params: any) => fmt1(params.value) } } },
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, gridIndex: 4, position: 'left', axisLabel: { show: false }, splitNumber: 3, name: 'BOLL', nameLocation: 'middle', nameRotate: 0, nameGap: 32, nameTextStyle: { color: '#94a3b8', fontSize: 11, align: 'right' }, axisPointer: { label: { show: true, formatter: (params: any) => fmt2(params.value) } } },
      ],
      dataZoom: [
        { type: 'inside', xAxisIndex: [0, 1, 2, 3, 4], start: 0, end: 100, zoomLock: true },
      ],
      series: [
        {
          name: 'K线',
          type: 'candlestick',
          data: candleData,
          itemStyle: {
            color: colors.up,
            color0: colors.down,
            borderColor: colors.up,
            borderColor0: colors.down,
          },
          xAxisIndex: 0,
          yAxisIndex: 0,
          cursor: 'default',
        },
        {
          name: '换手率',
          type: 'bar',
          data: turnoverData,
          xAxisIndex: 1,
          yAxisIndex: 1,
          cursor: 'default',
        },
        { name: 'MA5', type: 'line', data: sliceDisplay(ma5), smooth: false, lineStyle: { color: colors.ma5, width: 1.5 }, symbol: 'none', xAxisIndex: 0, yAxisIndex: 0, cursor: 'default' },
        { name: 'MA10', type: 'line', data: sliceDisplay(ma10), smooth: false, lineStyle: { color: colors.ma10, width: 1.5 }, symbol: 'none', xAxisIndex: 0, yAxisIndex: 0, cursor: 'default' },
        { name: 'MA30', type: 'line', data: sliceDisplay(ma30), smooth: false, lineStyle: { color: colors.ma30, width: 1.5 }, symbol: 'none', xAxisIndex: 0, yAxisIndex: 0, cursor: 'default' },
        { name: 'MA60', type: 'line', data: sliceDisplay(ma60), smooth: false, lineStyle: { color: colors.ma60, width: 1.5 }, symbol: 'none', xAxisIndex: 0, yAxisIndex: 0, cursor: 'default' },
        { name: 'DIF', type: 'line', data: sliceDisplay(dif), smooth: true, lineStyle: { color: colors.macd }, symbol: 'none', xAxisIndex: 2, yAxisIndex: 2, cursor: 'default' },
        { name: 'DEA', type: 'line', data: sliceDisplay(dea), smooth: true, lineStyle: { color: colors.signal }, symbol: 'none', xAxisIndex: 2, yAxisIndex: 2, cursor: 'default' },
        {
          name: 'MACD', type: 'bar', data: sliceDisplay(hist).map(v => typeof v === 'number' ? {
            value: v,
            itemStyle: { color: v >= 0 ? colors.histPositive : colors.histNegative },
          } : '-'),
          xAxisIndex: 2, yAxisIndex: 2, cursor: 'default',
        },
        { name: 'RSI6', type: 'line', data: sliceDisplay(rsi6), smooth: true, lineStyle: { color: colors.rsi6, width: 1.5 }, symbol: 'none', xAxisIndex: 3, yAxisIndex: 3, connectNulls: false, cursor: 'default' },
        { name: 'RSI12', type: 'line', data: sliceDisplay(rsi12), smooth: true, lineStyle: { color: colors.rsi12, width: 1.5 }, symbol: 'none', xAxisIndex: 3, yAxisIndex: 3, connectNulls: false, cursor: 'default' },
        { name: 'RSI24', type: 'line', data: sliceDisplay(rsi24), smooth: true, lineStyle: { color: colors.rsi24, width: 1.5 }, symbol: 'none', xAxisIndex: 3, yAxisIndex: 3, connectNulls: false, cursor: 'default' },
        { name: '上轨', type: 'line', data: sliceDisplay(bbUpper), smooth: true, lineStyle: { color: colors.bbUpper }, symbol: 'none', xAxisIndex: 4, yAxisIndex: 4, connectNulls: false, cursor: 'default' },
        { name: '中轨', type: 'line', data: sliceDisplay(bbMid), smooth: true, lineStyle: { color: colors.bbMid, width: 2 }, symbol: 'none', xAxisIndex: 4, yAxisIndex: 4, connectNulls: false, cursor: 'default' },
        { name: '下轨', type: 'line', data: sliceDisplay(bbLower), smooth: true, lineStyle: { color: colors.bbLower }, symbol: 'none', xAxisIndex: 4, yAxisIndex: 4, connectNulls: false, cursor: 'default' },
      ],
    }

    chart.setOption(option)

    const handleResize = () => chart.resize()
    window.addEventListener('resize', handleResize)

    return () => {
      window.removeEventListener('resize', handleResize)
      chart.dispose()
      chartInstanceRef.current = null
    }
  }, [data, isExpanded])

  const [isLightTheme, setIsLightTheme] = useState(false)
  useEffect(() => {
    const check = () => setIsLightTheme(document.body.classList.contains('light'))
    check()
    const observer = new MutationObserver(check)
    observer.observe(document.body, { attributes: true, attributeFilter: ['class'] })
    return () => observer.disconnect()
  }, [])

  const fullscreenBg = isLightTheme ? '#f8fafc' : '#0f172a'
  const btnBg = isLightTheme ? 'rgba(255,255,255,0.9)' : 'rgba(30,41,59,0.9)'
  const btnText = isLightTheme ? '#1f2937' : '#e2e8f0'
  const hintText = isLightTheme ? '#94a3b8' : '#64748b'

  useEffect(() => {
    if (!isExpanded) return
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setIsExpanded(false)
    }
    window.addEventListener('keydown', handleKey)
    return () => window.removeEventListener('keydown', handleKey)
  }, [isExpanded])

  if (loading) return <div style={{ padding: 40, textAlign: 'center', color: '#64748b' }}>加载图表数据中...</div>
  if (data.length === 0) return <div style={{ padding: 40, textAlign: 'center', color: '#64748b' }}>暂无K线数据</div>

  return (
    <div style={{ width: '100%', height: '560px', position: 'relative' }}>
      <div style={{
        width: isExpanded ? '100vw' : '100%',
        height: isExpanded ? '100vh' : '100%',
        position: isExpanded ? 'fixed' : 'relative',
        top: 0, left: 0,
        zIndex: isExpanded ? 9999 : 1,
        backgroundColor: isExpanded ? fullscreenBg : 'transparent',
      }}>
        <div style={{
          position: 'absolute', top: 12, left: 12, zIndex: 10000,
          pointerEvents: 'none',
        }}>
          <span style={{ color: hintText, fontSize: 11 }}>
            {isExpanded ? '双击 / Esc 回到原来的样式' : '双击能扩展到全窗口'}
          </span>
        </div>
        {isExpanded && (
          <button onClick={() => setIsExpanded(false)} style={{
            position: 'absolute', top: 12, right: 12, zIndex: 10000,
            padding: '6px 14px', borderRadius: 4,
            border: '1px solid rgba(148,163,184,0.3)',
            background: btnBg, color: btnText,
            fontSize: 13, cursor: 'pointer',
          }}>
            退出全屏
          </button>
        )}
        <div ref={chartRef} className="unified-chart-container" style={{ width: '100%', height: '100%' }} />
      </div>
    </div>
  )
}
